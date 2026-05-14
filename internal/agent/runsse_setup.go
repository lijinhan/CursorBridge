package agent

import (
	"context"
	"io"
	"strings"
	"time"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"cursorbridge/internal/debuglog"
	"cursorbridge/internal/strutil"
)

// setupRunSSESession prepares the SSE streaming session: decodes the request,
// resolves the adapter, builds initial messages, and starts the keepalive
// goroutine. Returns setupResult on success, or writes an error and returns
// nil on failure. The caller is responsible for calling stopKeepalive().
func setupRunSSESession(
	ctx context.Context,
	reqBody []byte,
	contentType string,
	rawWriter io.Writer,
	resolve AdapterResolver,
) (setupResult, func()) {
	w := &lockedWriter{w: rawWriter}
	bid := &aiserverv1.BidiRequestId{}
	if err := decodeUnary(reqBody, contentType, bid); err != nil {
		writeEndStreamError(w, "decode bidi request id: "+err.Error())
		return setupResult{}, func() {}
	}
	requestID := bid.GetRequestId()
	sess := WaitForSession(ctx, requestID)
	if sess != nil {
		debuglog.Printf("[RUNSSE] requestID=%s conversationID=%s userText=%q mode=%v",
			requestID, sess.ConversationID, strutil.Truncate(sess.UserText, 80), sess.Mode)
	}
	// When the session has a ConversationID but no UserText (continuation
	// round or prewarm), try to fill UserText from on-disk history.
	if sess != nil && sess.UserText == "" && sess.ConversationID != "" {
		if turns := HistoryFor(sess.ConversationID); len(turns) > 0 {
			last := turns[len(turns)-1]
			if last.User != "" {
				sess.UserText = last.User
				debuglog.Printf("[RUNSSE] filled UserText from history conversationID=%s", sess.ConversationID)
				PutSession(sess)
			}
		}
	}
	if sess == nil || sess.UserText == "" {
		writeEndStreamError(w, "no session/user text for request_id="+requestID)
		return setupResult{}, func() {}
	}

	// Start keepalive goroutine to prevent Cursor from closing the SSE pipe
	// after ~60-90s of write-side silence.
	keepaliveCtx, stopKeepalive := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-keepaliveCtx.Done():
				return
			case <-ticker.C:
				if err := writeKeepaliveFrame(w); err != nil {
					return
				}
			}
		}
	}()

	target, ok := ResolveAdapterForAgentSession(resolve.Resolve(), sess)
	if !ok {
		writeEndStreamError(w, "no BYOK adapter configured")
		stopKeepalive()
		return setupResult{}, func() {}
	}
	providerType, baseURL, apiKey, model, adapterOpts :=
		target.ProviderType, target.BaseURL, target.APIKey, target.Model, target.Opts

	var stream providerStreamer = streamOpenAI
	if strings.EqualFold(providerType, "anthropic") {
		stream = streamAnthropic
	}

	// Persist user message immediately so history exists before streaming.
	earlyPersistUserTurn(sess.ConversationID, requestID, sess.UserText)

	// Build initial context for the streaming loop.
	// openAIToolsForRequest populates sess.McpMap as a side effect.
	_ = openAIToolsForRequest(sess)
	messages := buildMessageHistory(sess)
	initialLen := len(messages)
	startedAt := time.Now()

	maxLoopRounds := adapterOpts.MaxLoopRounds
	maxTurnDurationSec := int64(0)
	if adapterOpts.MaxTurnDurationSec > 0 {
		maxTurnDurationSec = int64(adapterOpts.MaxTurnDurationSec)
	}
	turnDeadline := int64(0)
	if maxTurnDurationSec > 0 {
		turnDeadline = startedAt.Unix() + maxTurnDurationSec
	}

	return setupResult{
		w:               w,
		sess:            sess,
		requestID:       requestID,
		messages:        messages,
		initialLen:      initialLen,
		startedAt:       startedAt.Unix(),
		stream:          stream,
		adapterOpts:     adapterOpts,
		model:           model,
		baseURL:         baseURL,
		apiKey:          apiKey,
		maxLoopRounds:   maxLoopRounds,
		maxTurnDuration: maxTurnDurationSec,
		turnDeadline:    turnDeadline,
	}, stopKeepalive
}