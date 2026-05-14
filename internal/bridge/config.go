package bridge

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"cursorbridge/internal/relay"
	"cursorbridge/internal/safefile"
)

func readConfig(dir string) (UserConfig, error) {
	cfg := UserConfig{BaseURL: defaultUpstream}
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	_ = json.Unmarshal(b, &cfg)
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultUpstream
	}
	return cfg, nil
}


// adapterListFor reads the user config and returns the BYOK adapter list
// suitable for relay rewrites and SQLite pinning.
func adapterListFor(cfgDir string) []relay.AdapterInfo {
	c, err := readConfig(cfgDir)
	if err != nil {
		return nil
	}
	return adapterListFromConfig(c)
}

func adapterListFromConfig(c UserConfig) []relay.AdapterInfo {
	out := make([]relay.AdapterInfo, 0, len(c.ModelAdapters))
	maxTurnDurationSec := 0
	if c.MaxTurnDurationMin > 0 {
		maxTurnDurationSec = c.MaxTurnDurationMin * 60
	}
	for _, a := range c.ModelAdapters {
		if a.ModelID == "" {
			continue
		}
		out = append(out, relay.AdapterInfo{
			DisplayName:        a.DisplayName,
			Type:               a.Type,
			ModelID:            a.ModelID,
			BaseURL:            a.BaseURL,
			APIKey:             a.APIKey,
			ReasoningEffort:    a.ReasoningEffort,
			ServiceTier:        a.ServiceTier,
			MaxOutputTokens:    a.MaxOutputTokens,
			ThinkingBudget:     a.ThinkingBudget,
			RetryCount:         a.RetryCount,
			RetryIntervalMs:    a.RetryInterval,
			TimeoutMs:          a.Timeout,
			MaxLoopRounds:      c.MaxLoopRounds,
			MaxTurnDurationSec: maxTurnDurationSec,
			ToolExecTimeoutSec: c.ToolExecTimeoutSec,
			ContextTokenLimit:  parseContextWindow(a.ContextWindow),
		})
	}
	return prioritizeActiveAdapter(out, strings.TrimSpace(c.ActiveModelID))
}

func prioritizeActiveAdapter(adapters []relay.AdapterInfo, activeModelID string) []relay.AdapterInfo {
	if len(adapters) < 2 || activeModelID == "" {
		return adapters
	}
	idx := -1
	for i, a := range adapters {
		if strings.EqualFold(strings.TrimSpace(a.ModelID), activeModelID) {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return adapters
	}
	out := make([]relay.AdapterInfo, 0, len(adapters))
	out = append(out, adapters[idx])
	out = append(out, adapters[:idx]...)
	out = append(out, adapters[idx+1:]...)
	return out
}

func selectedModelFor(cfgDir string, purpose string) string {
	c, err := readConfig(cfgDir)
	if err != nil {
		return ""
	}
	switch purpose {
	case "commit":
		return strings.TrimSpace(c.CommitModelID)
	case "review":
		return strings.TrimSpace(c.ReviewModelID)
	default:
		return ""
	}
}

func (s *ProxyService) LoadUserConfig() (UserConfig, error) {
	return readConfig(s.cfgDir)
}

func (s *ProxyService) SaveUserConfig(cfg UserConfig) error {
	cfg.ActiveModelID = strings.TrimSpace(cfg.ActiveModelID)
	if cfg.ActiveModelID != "" {
		found := false
		for _, a := range cfg.ModelAdapters {
			if strings.EqualFold(strings.TrimSpace(a.ModelID), cfg.ActiveModelID) {
				found = true
				break
			}
		}
		if !found {
			cfg.ActiveModelID = ""
		}
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return safefile.Write(filepath.Join(s.cfgDir, "config.json"), b, 0o600)
}

func (s *ProxyService) SetBaseURL(url string) (ProxyState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if url == "" {
		return s.state, errBaseURLEmpty
	}
	s.state.BaseURL = url
	return s.state, nil
}

func (s *ProxyService) ConfigDir() string {
	return s.cfgDir
}

func (s *ProxyService) OpenSettingsFolder() error {
	path := s.cfgDir
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// parseContextWindow parses the contextWindow string field into a token limit.
// Accepts formats like "128k", "128000", "200K". Returns 0 for empty/invalid.
func parseContextWindow(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	multiplier := 1
	lower := strings.ToLower(s)
	if strings.HasSuffix(lower, "k") {
		multiplier = 1024
		s = s[:len(s)-1]
	}
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return v * multiplier
}
