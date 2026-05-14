// Package mitm implements a local HTTPS man-in-the-middle proxy that
// intercepts Cursor editor requests and routes them to the agent layer.
package mitm

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"cursorbridge/internal/agent"
	"cursorbridge/internal/certs"
	"cursorbridge/internal/relay"

	"github.com/elazarl/goproxy"
)

type Server struct {
	addr string
	srv  *http.Server
	ln   net.Listener
	gw   *relay.Gateway
}

// agentResolver lets the agent package fetch the user's first BYOK adapter
// at request time. The bridge package wires this in so MITM doesn't need to
// know about UserConfig parsing — we just ask "give me a usable provider".
type AgentResolver = agent.AdapterResolver

func New(addr string, ca *certs.CA, gw *relay.Gateway, resolver AgentResolver, selectedModel func(string) string) (*Server, error) {
	tlsCert, err := tls.X509KeyPair(ca.CertPEM(), ca.KeyPEM())
	if err != nil {
		return nil, err
	}
	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, err
	}
	tlsCert.Leaf = leaf

	goproxy.GoproxyCa = tlsCert
	// TLS config that only advertises http/1.1 in ALPN.
	// Cursor's BidiAppend/RunSSE endpoints use HTTP/2 (gRPC-style), but
	// goproxy's H2Transport transparently proxies frames to the upstream
	// server — bypassing our BYOK handler entirely. Worse, H2Transport's
	// direct dial can create a loop back to the MITM proxy itself when
	// the system proxy is active. By only offering http/1.1 in ALPN,
	// Chromium falls back to HTTP/1.1 (its standard ALPN fallback), and
	// our request interceptor can process every request.
	tlsCfg := func(host string, ctx *goproxy.ProxyCtx) (*tls.Config, error) {
		cfg, err := goproxy.TLSConfigFromCA(&tlsCert)(host, ctx)
		if err != nil {
			return nil, err
		}
		cfg.NextProtos = []string{"http/1.1"}
		return cfg, nil
	}
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: tlsCfg}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: tlsCfg}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: tlsCfg}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: tlsCfg}

	p := goproxy.NewProxyHttpServer()
	p.Verbose = true
	p.Logger = log.New(os.Stderr, "[goproxy] ", log.LstdFlags)
	p.CertStore = certs.NewCertCache()
	p.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	// --- Path classification ---
	//
	// Synthetic paths: we return hand-rolled protobuf (BYOK model list, etc.)
	// instead of forwarding to the upstream.
	syntheticAPI2Paths := map[string]struct{}{
		"/aiserver.v1.AiService/AvailableModels":          {},
		"/aiserver.v1.AiService/GetDefaultModelNudgeData": {},
		"/aiserver.v1.AiService/GetDefaultModel":          {},
	}
	// Agent-handled paths: our BYOK layer processes these (LLM inference,
	// tool execution, etc.)
	agentAPI2Paths := map[string]struct{}{
		"/aiserver.v1.BidiService/BidiAppend": {},
		"/agent.v1.AgentService/RunSSE":       {},
		"/aiserver.v1.AiService/GetChatTitle":                {},
		"/aiserver.v1.AiService/GetTerminalCompletion":       {},
		"/aiserver.v1.AiService/GetChatSuggestions":          {},
		"/aiserver.v1.AiService/GetConversationSummary":      {},
		"/aiserver.v1.AiService/GenerateTldr":                {},
		"/aiserver.v1.AiService/LintExplanation":              {},
		"/aiserver.v1.AiService/StreamDiffReview":             {},
		"/aiserver.v1.AiService/CountTokens":                  {},
		"/aiserver.v1.AiService/WriteGitCommitMessage":         {},
		"/aiserver.v1.AiService/StreamBugBotAgenticSSE":        {},
		"/aiserver.v1.BackgroundComposerService/AddAsyncFollowupBackgroundComposer": {},
		"/aiserver.v1.BackgroundComposerService/GetBackgroundComposerStatus":        {},
		"/aiserver.v1.BackgroundComposerService/AttachBackgroundComposer":           {},
		"/aiserver.v1.BackgroundComposerService/StreamInteractionUpdatesSSE":        {},
	}
	// Blocked paths: must return 404 to prevent Cursor's real backend from
	// overriding our BYOK injection. These return account/subscription data
	// that would tell Cursor "BYOK not allowed for your account" and hide
	// our injected models from the chat picker.
	// Telemetry/analytics paths are handled separately by isTelemetryPath().
	blockedAPI2Paths := map[string]struct{}{
		"/aiserver.v1.AiService/GetSubscriptionState":   {},
		"/aiserver.v1.AiService/GetPlanInfo":            {},
		"/aiserver.v1.AiService/GetFeatureFlags":        {},
		"/aiserver.v1.AiService/GetUsageLimits":         {},
		"/aiserver.v1.AiService/GetStripePortalUrl":     {},
		"/aiserver.v1.AiService/GetBillingInfo":         {},
		"/aiserver.v1.AiService/GetTeamInfo":            {},
		"/aiserver.v1.AiService/GetTeamSubscription":    {},
		"/aiserver.v1.AiService/GetProTrialEligibility": {},
		"/aiserver.v1.AiService/GetOnboardingState":     {},
		"/aiserver.v1.AiService/GetUserSettings":        {},
		"/aiserver.v1.AiService/GetModelUsageLimits":    {},
		"/aiserver.v1.AiService/GetFastPremiumUsage":    {},
		"/aiserver.v1.AiService/GetSlowPremiumUsage":    {},
		"/aiserver.v1.AiService/GetPerModelUsage":       {},
		"/aiserver.v1.AiService/GetUsageOverview":       {},
		"/aiserver.v1.AiService/GetHardLimitUsage":      {},
		"/aiserver.v1.AiService/GetReferralInfo":        {},
		"/aiserver.v1.AiService/GetProUpgradeNudge":     {},
		"/aiserver.v1.AiService/GetUpgradePrompt":       {},
		"/aiserver.v1.AiService/GetNudgeData":           {},
		"/aiserver.v1.AiService/GetPaywallState":        {},
		"/aiserver.v1.AiService/GetAiderConfig":         {},
		"/aiserver.v1.AiService/GetMcpServers":          {},
		"/aiserver.v1.AiService/GetNotepad":             {},
		"/aiserver.v1.AiService/GetUserStatus":          {},
		"/aiserver.v1.AiService/GetUserPreferences":     {},
		"/aiserver.v1.AiService/GetLspTools":            {},
		"/aiserver.v1.AiService/GetPlugins":             {},
		"/aiserver.v1.AiService/GetProfile":             {},
		"/aiserver.v1.AiService/GetAuthProfile":         {},
		"/aiserver.v1.AiService/GetUserAccountInfo":     {},
		"/aiserver.v1.AiService/GetCursorConfig":        {},
		"/aiserver.v1.AiService/GetRemoteConfig":        {},
		"/aiserver.v1.AiService/GetCodebaseIndexingStatus": {},
		"/aiserver.v1.AiService/GetCodebaseIndexingConfig": {},
		"/aiserver.v1.AiService/GetCodebaseSyncStatus":  {},
		"/aiserver.v1.AiService/GetCodebaseSyncConfig":  {},
	}
	authHosts := map[string]struct{}{
		"prod.authentication.cursor.sh":     {},
		"prod.authentication.cursor.sh:443": {},
		"authentication.cursor.sh":          {},
		"authentication.cursor.sh:443":      {},
	}
	api2Hosts := map[string]struct{}{
		"api2.cursor.sh":     {},
		"api2.cursor.sh:443": {},
	}
	// Known external telemetry/analytics hosts that Cursor may phone home to.
	// Block these to prevent any user data, usage metrics, or conversation
	// content from reaching third-party analytics services.
	blockedTelemetryHosts := map[string]struct{}{
		"telemetry.cursor.sh":          {},
		"telemetry.cursor.sh:443":      {},
		"stats.cursor.sh":             {},
		"stats.cursor.sh:443":         {},
		"app.posthog.com":             {},
		"app.posthog.com:443":         {},
		"us.posthog.com":              {},
		"us.posthog.com:443":          {},
		"eu.posthog.com":              {},
		"eu.posthog.com:443":          {},
		"o0.ingest.sentry.io":         {},
		"o0.ingest.sentry.io:443":     {},
		"sentry.io":                   {},
		"sentry.io:443":               {},
		"api.amplitude.com":           {},
		"api.amplitude.com:443":       {},
		"api.segment.io":              {},
		"api.segment.io:443":          {},
		"cdn.segment.com":             {},
		"cdn.segment.com:443":         {},
		"api.mixpanel.com":            {},
		"api.mixpanel.com:443":        {},
		"statsigapi.net":              {},
		"statsigapi.net:443":          {},
		"featuregates.org":            {},
		"featuregates.org:443":        {},
		"events.launchdarkly.com":     {},
		"events.launchdarkly.com:443": {},
	}
	p.OnRequest().DoFunc(func(req *http.Request, _ *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		// HTTP/2 PRI requests arrive with an incomplete URL (just the
		// method, no host/path). Let goproxy's H2Transport handle them
		// rather than crashing on req.URL.Host.
		if req.URL == nil || req.Method == "PRI" {
			return req, nil
		}
		host := req.URL.Host
		path := req.URL.Path
		_, isAuthHost := authHosts[host]
		_, isAPI2Host := api2Hosts[host]
		if isAPI2Host {
			// 1. Synthetic paths → return mock proto (BYOK model list, etc.)
			if _, ok := syntheticAPI2Paths[path]; ok && gw != nil {
				if body := gw.SyntheticPath(path); body != nil {
					return req, mockProto(req, body)
				}
			}
			// 1.5 Structured synthetic paths → return typed proto responses
			// (IsConnected, GetMe, GetEmail, etc.) These need real fields
			// rather than empty bodies so Cursor's UI renders correctly.
			if resp := handleSyntheticPath(req, path); resp != nil {
				return req, resp
			}
			// 2. Agent-handled paths → BYOK layer processes them
			if _, ok := agentAPI2Paths[path]; ok {
				if path == "/aiserver.v1.BidiService/BidiAppend" {
					return req, handleBidiAppend(req)
				}
				if path == "/agent.v1.AgentService/RunSSE" {
					return req, handleRunSSE(req, resolver)
				}
				if path == "/aiserver.v1.AiService/WriteGitCommitMessage" {
					return req, handleWriteGitCommitMessage(req, resolver, selectedModel("commit"))
				}
				if path == "/aiserver.v1.AiService/StreamBugBotAgenticSSE" {
					return req, handleBugBotRunSSE(req, resolver, selectedModel("review"))
				}
				if path == "/aiserver.v1.BackgroundComposerService/AddAsyncFollowupBackgroundComposer" {
					return req, handleBackgroundComposerAddFollowup(req)
				}
				if path == "/aiserver.v1.BackgroundComposerService/GetBackgroundComposerStatus" {
					return req, handleBackgroundComposerStatus(req)
				}
				if path == "/aiserver.v1.BackgroundComposerService/AttachBackgroundComposer" {
					return req, handleBackgroundComposerAttach(req, resolver)
				}
				if path == "/aiserver.v1.BackgroundComposerService/StreamInteractionUpdatesSSE" {
					return req, handleBackgroundComposerInteractionUpdates(req)
				}
			}
			// 3. Blocked paths → 404 (prevent BYOK override from upstream)
			if _, ok := blockedAPI2Paths[path]; ok {
				return req, mock404(req)
			}
			// 3.5 KnowledgeBase paths → local storage for Rules persistence.
			if strings.HasPrefix(path, "/aiserver.v1.AiService/KnowledgeBase") {
				return req, handleKnowledgeBase(req, path)
			}
			// 4. Telemetry/analytics paths → 404. These send conversation
			// content, usage data, and metrics to Cursor's servers. Block
			// them to prevent leaking user data through the BYOK proxy.
			// Matched by method-name prefix patterns from the proto spec.
			if isTelemetryPath(path) {
				return req, mock404(req)
			}
			// 5. DashboardService/AnalyticsService paths → return empty success.
			// Without a valid Cursor Pro account these return 401 from upstream,
			// which can break Cursor's UI (e.g. Rules panel won't load).
			if isEmptyMockPath(path) {
				return req, mockProto(req, nil)
			}
			// 6. All remaining api2 paths → return empty 200 proto.
			// Without a valid Cursor Pro subscription most remaining
			// service calls (NetworkService, RepositoryService,
			// FileSyncService, ServerConfigService, AuthService, etc.)
			// return 401 Unauthorized from upstream. This triggers
			// Cursor to show a login prompt. Instead we mock success
			// with an empty protobuf body so Cursor considers itself
			// "connected" and never forces re-login.
			log.Printf("[MITM] mock-200: %s%s", host, path)
			return req, mockProto(req, nil)
		}
		if isAuthHost {
			// Auth host requests: return empty 200 to prevent login loops.
			// The real auth server would return tokens, but since we don't
			// need actual Cursor auth (BYOK mode), an empty response
			// tells Cursor the auth flow completed without error.
			return req, mockProto(req, nil)
		}
		// Block known telemetry/analytics hosts.
		if _, blocked := blockedTelemetryHosts[host]; blocked {
			log.Printf("[MITM] blocked telemetry: %s%s", host, path)
			return req, mock404(req)
		}
		// Block ALL remaining *.cursor.sh subdomains — nothing should
		// reach Cursor's servers. This is a security catch-all to
		// prevent any data exfiltration we haven't explicitly handled.
		hostNoPort := host
		if idx := strings.LastIndex(host, ":"); idx != -1 {
			hostNoPort = host[:idx]
		}
		if strings.HasSuffix(hostNoPort, ".cursor.sh") || hostNoPort == "cursor.sh" {
			log.Printf("[MITM] blocked cursor domain: %s%s", host, path)
			return req, mock404(req)
		}
		return req, nil
	})

	if gw != nil {
		p.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			var req *http.Request
			if ctx != nil {
				req = ctx.Req
			}
			gw.MaybeRewriteResponse(req, resp)
			return resp
		})
	}

	return &Server{
		addr: addr,
		gw:   gw,
		srv:  &http.Server{Handler: p, ReadHeaderTimeout: 30 * time.Second},
	}, nil
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.ln = ln
	go func() { _ = s.srv.Serve(ln) }()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}

