package agent

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"cursorbridge/internal/debuglog"
	"cursorbridge/internal/strutil"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"
	"cursorbridge/internal/relay"

	"google.golang.org/protobuf/proto"
)

// execSeqCounter hands out monotonically increasing ExecServerMessage.Id
// values. Previously we derived seq from (round*10 + len(result.ToolCalls) + 1)
// — but len(result.ToolCalls) is constant across the inner loop, so every
// tool call in a single round received the SAME seq. That made seqAlias
// and shellAccum collide: a second tool call's shell Start event could
// land on the first call's accumulator, and backgrounded shell events
// arriving late could wake the wrong pending waiter.
//
// Starting at 1 so a zero-valued seq can be distinguished from "real" ids
// during debugging.
var execSeqCounter atomic.Uint32

func nextExecSeq() uint32 {
	return execSeqCounter.Add(1)
}

// lockedWriter serialises all frame writes against a single mutex so the
// background keepalive goroutine and the main response loop can share one
// pipe without interleaving bytes between a frame's header and its body.
// Once a write fails (client disconnected), all subsequent writes return
// the cached error immediately so the caller can detect the broken pipe
// and abort the agent loop instead of continuing to wait for tool results.
type lockedWriter struct {
	mu   sync.Mutex
	w    io.Writer
	err  error // sticky write error — once set, all writes fail fast
	done bool   // true after first write error
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.done {
		return 0, l.err
	}
	n, err := l.w.Write(p)
	if err != nil {
		l.done = true
		l.err = err
	}
	return n, err
}

// Broken reports whether the underlying writer has seen a write error,
// meaning the client has disconnected and no further frames will be
// delivered. The agent loop checks this to decide whether to abort early.
func (l *lockedWriter) Broken() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.done
}

func (l *lockedWriter) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if f, ok := l.w.(interface{ Flush() }); ok {
		f.Flush()
	}
}

type AdapterTarget struct {
	ProviderType string
	BaseURL      string
	APIKey       string
	Model        string
	StableID     string
	DisplayName  string
	Opts         AdapterOpts
}

// AdapterResolver is the interface the agent package uses to fetch the user's
// configured BYOK adapters at request time. The bridge implements it so MITM
// doesn't need to know about UserConfig parsing.
type AdapterResolver interface {
	Resolve() []AdapterTarget
}

// AdapterResolverFunc adapts a plain function to AdapterResolver.
type AdapterResolverFunc func() []AdapterTarget

func (f AdapterResolverFunc) Resolve() []AdapterTarget { return f() }

type AdapterOpts struct {
	ReasoningEffort    string
	ServiceTier        string
	MaxOutputTokens    int
	ThinkingBudget     int
	RetryCount         int
	RetryIntervalMs    int
	TimeoutMs          int
	MaxLoopRounds      int
	MaxTurnDurationSec int
	ToolExecTimeoutSec int
	ContextTokenLimit  int
}

func AdapterTargetFromRelay(a relay.AdapterInfo) AdapterTarget {
	return AdapterTarget{
		ProviderType: a.Type,
		BaseURL:      a.BaseURL,
		APIKey:       a.APIKey,
		Model:        a.ModelID,
		StableID:     a.StableID(),
		DisplayName:  a.DisplayName,
		Opts: AdapterOpts{
			ReasoningEffort:    a.ReasoningEffort,
			ServiceTier:        a.ServiceTier,
			MaxOutputTokens:    a.MaxOutputTokens,
			ThinkingBudget:     a.ThinkingBudget,
			RetryCount:         a.RetryCount,
			RetryIntervalMs:    a.RetryIntervalMs,
			TimeoutMs:          a.TimeoutMs,
			MaxLoopRounds:      a.MaxLoopRounds,
			MaxTurnDurationSec: a.MaxTurnDurationSec,
			ToolExecTimeoutSec: a.ToolExecTimeoutSec,
			ContextTokenLimit:  a.ContextTokenLimit,
		},
	}
}

func ResolveAdapterForModel(adapters []AdapterTarget, requested string) (AdapterTarget, bool) {
	if len(adapters) == 0 {
		return AdapterTarget{}, false
	}
	requested = strings.TrimSpace(requested)
	if requested != "" {
		for _, a := range adapters {
			if strings.EqualFold(requested, a.StableID) || strings.EqualFold(requested, a.Model) || (a.DisplayName != "" && strings.EqualFold(requested, a.DisplayName)) {
				return a, true
			}
		}
	}
	// The adapter slice arrives priority-sorted by the bridge, with the
	// configured active model first.
	return adapters[0], true
}

func ResolveAdapterForAgentSession(adapters []AdapterTarget, sess *Session) (AdapterTarget, bool) {
	requested := ""
	if sess != nil && sess.ModelDetails != nil {
		requested = sess.ModelDetails.GetModelId()
		if requested == "" {
			requested = sess.ModelDetails.GetDisplayModelId()
		}
	}
	return ResolveAdapterForModel(adapters, requested)
}

func ResolveAdapterForBugBotRequest(adapters []AdapterTarget, req *aiserverv1.StreamBugBotRequest) (AdapterTarget, bool) {
	requested := ""
	if req != nil && req.GetModelDetails() != nil {
		requested = req.GetModelDetails().GetModelName()
	}
	return ResolveAdapterForModel(adapters, requested)
}

// RunSSEHeaders is the response header set the bridge writes BEFORE handing
// the body pipe to RunSSE. They match the working app's captured
// agent.v1.AgentService/RunSSE response: text/event-stream content type and
// Connect's gzip negotiation hints.
var RunSSEHeaders = http.Header{
	"Content-Type":             {"text/event-stream"},
	"Cache-Control":            {"no-cache"},
	"Connect-Content-Encoding": {"gzip"},
	"Connect-Accept-Encoding":  {"gzip"},
}

// HandleRunSSE streams the BYOK chat completion to w as Connect SSE frames.
// The MITM bridge already wrote response headers and a 200 status; this
// function only writes the body. Returns when the stream finishes (success
// or error), at which point the bridge can close the underlying pipe.
func HandleRunSSE(
	ctx context.Context,
	reqBody []byte,
	contentType string,
	rawWriter io.Writer,
	resolve AdapterResolver,
	deps *AgentDeps,
) {
	// Setup: decode request, resolve adapter, start keepalive.
	setup, stopKeepalive := setupRunSSESession(ctx, reqBody, contentType, rawWriter, resolve, deps)
	defer stopKeepalive()
	if setup.w == nil {
		return // setupRunSSESession wrote the error
	}

	// Run the main streaming loop.
	res := runStreamingLoop(ctx, setup)

	// Skip finalizeTurn on early exit (client disconnected or stream error
	// already persisted the partial turn in runStreamingLoop).
	if res.earlyExit {
		return
	}

	// Finalize: cap warnings, persistence, session cleanup.
	finalizeTurn(setup, res)
}

// providerFromURL guesses a human-readable provider slug from the baseURL.
// providerFromURL guesses a human-readable provider slug from the baseURL.
func providerFromURL(url string) string {
	url = strings.ToLower(url)
	switch {
	case strings.Contains(url, "anthropic.com"):
		return "anthropic"
	case strings.Contains(url, "openai.com"):
		return "openai"
	case strings.Contains(url, "generativelanguage.googleapis.com"):
		return "gemini"
	case strings.Contains(url, "deepseek.com"):
		return "deepseek"
	case strings.Contains(url, "mistral.ai"):
		return "mistral"
	case strings.Contains(url, "localhost:11434"):
		return "ollama"
	case strings.Contains(url, "openrouter.ai"):
		return "openrouter"
	case strings.Contains(url, "groq.com"):
		return "groq"
	case strings.Contains(url, "together"):
		return "together"
	default:
		return "custom"
	}
}

func requestedModelForSession(sess *Session) string {
	if sess == nil || sess.ModelDetails == nil {
		return ""
	}
	if id := sess.ModelDetails.GetModelId(); id != "" {
		return id
	}
	return sess.ModelDetails.GetDisplayModelId()
}

// persistPartialTurn saves whatever work the agent loop accumulated before the
// SSE connection broke or a write failed. Without this, a mid-turn disconnect
// loses all tool-call history and assistant text, causing Cursor to rollback
// the entire turn on reconnect.
func persistPartialTurn(
	sess *Session, requestID string, startedAt int64,
	messages []openAIMessage, initialLen int,
	model, baseURL string,
	assistantBuf *strings.Builder,
	lastResult *streamResult,
	promptTokens, completionTokens int64,
) {
	if sess == nil {
		return
	}
	assistantText := assistantBuf.String()
	var storedMessages []StoredMessage
	if len(messages) > initialLen {
		storedMessages = make([]StoredMessage, 0, len(messages)-initialLen)
		for _, m := range messages[initialLen:] {
			storedMessages = append(storedMessages, openAIToStored(m))
		}
	}
	// Skip recording turns that produced no meaningful content.
	hasToolHistory := len(storedMessages) > 0
	hasMeaningfulText := len(assistantText) >= 100 || (len(assistantText) > 0 && hasToolHistory)
	if !hasMeaningfulText && !hasToolHistory {
		debuglog.Printf("[RUNSSE] persistPartialTurn: SKIPPING empty turn")
		return
	}
	if assistantText == "" && lastResult != nil && len(lastResult.ToolCalls) > 0 {
		var names []string
		for _, tc := range lastResult.ToolCalls {
			names = append(names, tc.Name)
		}
		assistantText = "[partial turn: " + strings.Join(names, ", ") + "]"
	}
	if assistantText == "" {
		assistantText = "[partial turn: disconnected]"
	}
	RecordTurn(sess.ConversationID, requestID, sess.UserText, assistantText, modeString(sess.Mode), nil, storedMessages)
	debuglog.Printf("[RUNSSE] persistPartialTurn: saved conversationID=%s requestID=%s", sess.ConversationID, requestID)
}

// writeEndStreamError emits the Connect END-STREAM frame with an error payload.
func writeEndStreamError(w io.Writer, msg string) {
	body := []byte(`{"error":{"code":"internal","message":` + strutil.JSONString(msg) + `}}`)
	hdr := [5]byte{flagEndStream, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(hdr[1:5], uint32(len(body)))
	_, _ = w.Write(hdr[:])
	_, _ = w.Write(body)
	flushIfPossible(w)
}

// Compile-time guard so I notice if proto stops being used here in a future
// refactor.
var _ = proto.Marshal
type setupResult struct {
	deps            *AgentDeps
	w               *lockedWriter
	sess            *Session
	requestID       string
	messages        []openAIMessage
	initialLen      int
	startedAt       int64 // Unix timestamp
	stream          providerStreamer
	adapterOpts     AdapterOpts
	model           string
	baseURL         string
	apiKey          string
	maxLoopRounds   int
	maxTurnDuration int64 // seconds
	turnDeadline    int64 // Unix timestamp
}

// loopResult holds everything runStreamingLoop returns.
type loopResult struct {
	messages         []openAIMessage
	initialLen       int
	assistantBuf     strings.Builder
	lastResult       *streamResult
	promptTokens     int64
	completionTokens int64
	round            int
	capped           bool
	earlyExit        bool
}