package bridge

import (
	"encoding/json"
	"strconv"
)

type ProxyState struct {
	Running       bool   `json:"running"`
	ListenAddr    string `json:"listenAddr"`
	BaseURL       string `json:"baseURL"`
	StartedAt     int64  `json:"startedAt"`
	CAFingerprint string `json:"caFingerprint"`
	CAPath        string `json:"caPath"`
	CAInstalled   bool   `json:"caInstalled"`
	CAInstallMode string `json:"caInstallMode,omitempty"`
	CAWarning     string `json:"caWarning,omitempty"`
	LastError     string `json:"lastError,omitempty"`
}

type ModelAdapterConfig struct {
	DisplayName     string `json:"displayName"`
	Type            string `json:"type"`
	BaseURL         string `json:"baseURL"`
	APIKey          string `json:"apiKey"`
	ModelID         string `json:"modelID"`
	ContextWindow   string `json:"contextWindow,omitempty"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
	ServiceTier     string `json:"serviceTier,omitempty"`
	MaxOutputTokens int    `json:"maxOutputTokens,omitempty"`
	ThinkingBudget  int    `json:"thinkingBudget,omitempty"`
	RetryCount      int    `json:"retryCount,omitempty"`
	RetryInterval   int    `json:"retryInterval,omitempty"`
	Timeout         int    `json:"timeout,omitempty"`
	Notes           string `json:"notes,omitempty"`
	LastTestResult  string `json:"lastTestResult,omitempty"`
	LastTestedAt    int64  `json:"lastTestedAt,omitempty"`
}

// UnmarshalJSON implements custom deserialization that tolerates both JSON
// numbers and JSON strings for the integer fields. This is needed because
// older config.json files stored these as strings (e.g. "3" instead of 3)
// while the frontend now sends numbers.
func (m *ModelAdapterConfig) UnmarshalJSON(data []byte) error {
	type Alias ModelAdapterConfig
	aux := &struct {
		MaxOutputTokens json.RawMessage `json:"maxOutputTokens,omitempty"`
		ThinkingBudget  json.RawMessage `json:"thinkingBudget,omitempty"`
		RetryCount      json.RawMessage `json:"retryCount,omitempty"`
		RetryInterval   json.RawMessage `json:"retryInterval,omitempty"`
		Timeout         json.RawMessage `json:"timeout,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	m.MaxOutputTokens = rawToInt(aux.MaxOutputTokens)
	m.ThinkingBudget = rawToInt(aux.ThinkingBudget)
	m.RetryCount = rawToInt(aux.RetryCount)
	m.RetryInterval = rawToInt(aux.RetryInterval)
	m.Timeout = rawToInt(aux.Timeout)
	return nil
}

// rawToInt parses a JSON raw message as an int, accepting both numbers and
// quoted strings. Returns 0 for nil/empty input.
func rawToInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	// Try as number first.
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	// Try as string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" {
			return 0
		}
		v, err := strconv.Atoi(s)
		if err != nil {
			return 0
		}
		return v
	}
	return 0
}

type UserConfig struct {
	BaseURL       string               `json:"baseURL"`
	ModelAdapters []ModelAdapterConfig `json:"modelAdapters"`
	ActiveModelID string               `json:"activeModelID,omitempty"`
	CommitModelID string               `json:"commitModelID,omitempty"`
	ReviewModelID string               `json:"reviewModelID,omitempty"`
	// CloseAction remembers what the user picked in the "Quit or minimize
	// to tray?" dialog. Empty = never answered, show the dialog on close.
	// "quit" = shut the proxy down and exit. "tray" = hide to system tray.
	CloseAction string `json:"closeAction,omitempty"`
	// MaxLoopRounds caps the agent tool-call loop rounds. 0 = no cap (native
	// Cursor behaviour — the client controls when to stop). A positive value
	// limits each agentic turn to that many LLM round-trips.
	MaxLoopRounds int `json:"maxLoopRounds,omitempty"`
	// MaxTurnDurationMin caps the wall-clock time per agentic turn. 0 = no
	// cap (native behaviour). A positive value sets the cap in minutes.
	MaxTurnDurationMin int `json:"maxTurnDurationMin,omitempty"`
	// ToolExecTimeoutSec caps how long the agent waits for Cursor's IDE to
	// execute a tool and return the result. 0 = use defaults (30s for agent
	// and BugBot, 2min for BackgroundComposer).
	ToolExecTimeoutSec int `json:"toolExecTimeoutSec,omitempty"`
}

// UsageStats is the aggregate token-use snapshot the Stats tab in the Wails
// frontend displays. It's computed on demand from the on-disk history root
// (%APPDATA%/cursorbridge/history/<conversationID>/turns/*/summary.json),
// so the values always reflect the complete recorded conversation corpus —
// not just the live in-memory session window.
type UsageStats struct {
	TotalPromptTokens     int64             `json:"totalPromptTokens"`
	TotalCompletionTokens int64             `json:"totalCompletionTokens"`
	TotalTokens           int64             `json:"totalTokens"`
	ConversationCount     int               `json:"conversationCount"`
	TurnCount             int               `json:"turnCount"`
	PerModel              []ModelUsageEntry `json:"perModel"`
	Last7Days             []DailyUsageEntry `json:"last7Days"`
}

type ModelUsageEntry struct {
	Model            string `json:"model"`
	Provider         string `json:"provider"`
	PromptTokens     int64  `json:"promptTokens"`
	CompletionTokens int64  `json:"completionTokens"`
	TurnCount        int    `json:"turnCount"`
}

type DailyUsageEntry struct {
	Date             string `json:"date"` // YYYY-MM-DD in local time
	PromptTokens     int64  `json:"promptTokens"`
	CompletionTokens int64  `json:"completionTokens"`
}

type CursorSettingsStatus struct {
	Path            string `json:"path"`
	Found           bool   `json:"found"`
	Error           string `json:"error,omitempty"`
	ProxySet        bool   `json:"proxySet"`
	ProxyValue      string `json:"proxyValue,omitempty"`
	StrictSSLOff    bool   `json:"strictSSLOff"`
	ProxySupportOn  bool   `json:"proxySupportOn"`
	SystemCertsV2On bool   `json:"systemCertsV2On"`
	UseHTTP1        bool   `json:"useHttp1"`
	DisableHTTP2    bool   `json:"disableHttp2"`
	ProxyKerberos   bool   `json:"proxyKerberos"`
}
