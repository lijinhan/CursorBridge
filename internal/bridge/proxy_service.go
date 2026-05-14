// Package bridge provides the Wails front-end binding layer, exposing
// Go backend methods (ProxyService) and shared types to the Vue UI.
package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cursorbridge/internal/agent"
	"cursorbridge/internal/appdir"
	"cursorbridge/internal/certs"
	"cursorbridge/internal/cursor"
	"cursorbridge/internal/mitm"
	"cursorbridge/internal/relay"
)

const (
	defaultProxyPort = 18080
	defaultUpstream   = "https://api2.cursor.sh"
)

// errBaseURLEmpty is the sentinel error returned when SetBaseURL receives
// an empty string. Defined here so config.go can reference it without
// importing a separate errmsg package (the bridge package is small enough
// that internal constants suffice).
var errBaseURLEmpty = errors.New("base URL cannot be empty")

type ProxyService struct {
	mu              sync.RWMutex
	state           ProxyState
	proxy           *mitm.Server
	ca              *certs.CA
	gateway         *relay.Gateway
	cfgDir          string
	onStateChange   func(running bool)
	gatewayAdapters func() []relay.AdapterInfo
	quitCb          func()
	hideCb          func()
}

func NewProxyService() (*ProxyService, error) {
	dir, err := appdir.ConfigDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ca, err := certs.LoadOrCreate(filepath.Join(dir, "ca"))
	if err != nil {
		return nil, err
	}
	gw := relay.NewGateway()
	agent.InitHistoryDir(filepath.Join(dir, "history"))
	cfg, _ := readConfig(dir)
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultUpstream
	}
	svc := &ProxyService{
		cfgDir:  dir,
		ca:      ca,
		gateway: gw,
		state: ProxyState{
			ListenAddr:    fmt.Sprintf("127.0.0.1:%d", defaultProxyPort),
			BaseURL:       baseURL,
			CAFingerprint: ca.Fingerprint(),
			CAPath:        filepath.Join(ca.Dir(), "ca.crt"),
			CAInstalled:   certs.IsInstalledUserRoot(),
			CAInstallMode: detectCAInstallMode(),
			CAWarning:     buildCAWarning(),
		},
	}
	svc.gatewayAdapters = func() []relay.AdapterInfo { return adapterListFor(svc.cfgDir) }
	gw.SetAdapterProvider(func() []relay.AdapterInfo {
		c, err := readConfig(svc.cfgDir)
		if err != nil {
			return nil
		}
		return adapterListFromConfig(c)
	})
	return svc, nil
}

func (s *ProxyService) GetState() ProxyState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.state
	st.CAInstalled = certs.IsInstalledUserRoot()
	st.CAInstallMode = detectCAInstallMode()
	st.CAWarning = buildCAWarning()
	return st
}

func (s *ProxyService) StartProxy() (ProxyState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Running {
		return s.state, nil
	}
	cfg, _ := readConfig(s.cfgDir)
	port := defaultProxyPort
	if cfg.ProxyPort > 0 && cfg.ProxyPort <= 65535 {
		port = cfg.ProxyPort
	}
	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)
	s.state.ListenAddr = listenAddr
	resolver := agent.AdapterResolverFunc(func() []agent.AdapterTarget {
		ads := s.gatewayAdapters()
		out := make([]agent.AdapterTarget, 0, len(ads))
		for _, a := range ads {
			out = append(out, agent.AdapterTargetFromRelay(a))
		}
		return out
	})
	selectedModel := func(purpose string) string { return selectedModelFor(s.cfgDir, purpose) }
	srv, err := mitm.New(listenAddr, s.ca, s.gateway, resolver, selectedModel)
	if err != nil {
		s.state.LastError = err.Error()
		return s.state, err
	}
	if err := srv.Start(); err != nil {
		s.state.LastError = err.Error()
		return s.state, err
	}
	if err := cursor.EnableSystemProxy(s.state.ListenAddr); err != nil {
		_ = srv.Stop(context.Background())
		s.state.LastError = "proxy started but failed to update system settings: " + err.Error()
		return s.state, err
	}
	var warnings []string
	if err := cursor.ApplyCursorTweaks(s.state.ListenAddr); err != nil {
		warnings = append(warnings, "Cursor settings.json tweak failed: "+err.Error())
	}
	if err := cursor.InjectFakeProUser(s.authBackupPath()); err != nil {
		warnings = append(warnings, "Cursor SQLite Pro injection failed: "+err.Error())
	}
	if adapters := s.gatewayAdapters(); len(adapters) > 0 {
		if err := cursor.ForceModelSelection(adapters[0].StableID()); err != nil {
			warnings = append(warnings, "Cursor SQLite model pin failed: "+err.Error())
		}
	}
	s.proxy = srv
	s.state.Running = true
	s.state.StartedAt = time.Now().Unix()
	if len(warnings) > 0 {
		s.state.LastError = "partial start: " + strings.Join(warnings, "; ")
	} else {
		s.state.LastError = ""
	}
	s.fireState()
	return s.state, nil
}

func (s *ProxyService) StopProxy() (ProxyState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.Running {
		return s.state, nil
	}
	_ = cursor.DisableSystemProxy()
	_ = cursor.RevertCursorTweaks()
	if err := cursor.RestoreFakeProUser(s.authBackupPath()); err != nil {
		s.state.LastError = "Cursor SQLite auth restore failed: " + err.Error()
	}
	if s.proxy != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.proxy.Stop(shutdownCtx)
		shutdownCancel()
		s.proxy = nil
	}
	s.state.Running = false
	s.state.StartedAt = 0
	s.fireState()
	return s.state, nil
}

func (s *ProxyService) authBackupPath() string {
	return filepath.Join(s.cfgDir, "cursor-auth-backup.json")
}

func (s *ProxyService) Shutdown() {
	_, _ = s.StopProxy()
}

func (s *ProxyService) GetCursorSettingsStatus() CursorSettingsStatus {
	st := cursor.GetStatus()
	return CursorSettingsStatus{
		Path:            st.Path,
		Found:           st.Found,
		Error:           st.Error,
		ProxySet:        st.ProxySet,
		ProxyValue:      st.ProxyValue,
		StrictSSLOff:    st.StrictSSLOff,
		ProxySupportOn:  st.ProxySupportOn,
		SystemCertsV2On: st.SystemCertsV2On,
		UseHTTP1:        st.UseHTTP1,
		DisableHTTP2:    st.DisableHTTP2,
		ProxyKerberos:   st.ProxyKerberos,
	}
}

func (s *ProxyService) ApplyCursorTweaks() (CursorSettingsStatus, error) {
	if err := cursor.ApplyCursorTweaks(s.state.ListenAddr); err != nil {
		return s.GetCursorSettingsStatus(), err
	}
	return s.GetCursorSettingsStatus(), nil
}

func (s *ProxyService) RevertCursorTweaks() (CursorSettingsStatus, error) {
	if err := cursor.RevertCursorTweaks(); err != nil {
		return s.GetCursorSettingsStatus(), err
	}
	return s.GetCursorSettingsStatus(), nil
}

// GetUsageStats returns the aggregate token-use snapshot the Stats tab in the
// Wails UI renders. Computed on demand from disk; safe to call any time.
func (s *ProxyService) GetUsageStats() UsageStats {
	snap := agent.ComputeUsageStats()
	out := UsageStats{
		TotalPromptTokens:     snap.TotalPromptTokens,
		TotalCompletionTokens: snap.TotalCompletionTokens,
		TotalTokens:           snap.TotalTokens,
		ConversationCount:     snap.ConversationCount,
		TurnCount:             snap.TurnCount,
		PerModel:              make([]ModelUsageEntry, 0, len(snap.PerModel)),
		Last7Days:             make([]DailyUsageEntry, 0, len(snap.Last7Days)),
	}
	for _, m := range snap.PerModel {
		out.PerModel = append(out.PerModel, ModelUsageEntry{
			Model:            m.Model,
			Provider:         m.Provider,
			PromptTokens:     m.PromptTokens,
			CompletionTokens: m.CompletionTokens,
			TurnCount:        m.TurnCount,
		})
	}
	for _, d := range snap.Last7Days {
		out.Last7Days = append(out.Last7Days, DailyUsageEntry{
			Date:             d.Date,
			PromptTokens:     d.PromptTokens,
			CompletionTokens: d.CompletionTokens,
		})
	}
	return out
}