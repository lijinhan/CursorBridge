package agent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"cursorbridge/internal/logutil"
	"cursorbridge/internal/strutil"
)

// runStreamingLoop is the main agent loop: calls the LLM, writes SSE frames,
// executes tools, and manages the message history. Returns when the loop
// terminates (stream ends, error, round cap, or client disconnect).
func runStreamingLoop(
	ctx context.Context,
	setup setupResult,
) loopResult {
	messages := setup.messages
	tools := openAIToolsForRequest(setup.sess)
	initialLen := setup.initialLen
	var assistantBuf strings.Builder
	var lastResult *streamResult
	var promptTokens, completionTokens int64

	for round := 0; setup.maxLoopRounds <= 0 || round < setup.maxLoopRounds; round++ {
		// Check time cap.
		if setup.maxTurnDuration > 0 && time.Now().Unix() > setup.turnDeadline {
			break
		}
		// Check client disconnect.
		if setup.w.Broken() {
			logutil.Debug("client disconnected, aborting at round", "round", round)
			persistPartialTurn(setup.sess, setup.requestID, setup.startedAt, messages, initialLen,
				setup.model, setup.baseURL, &assistantBuf, lastResult, promptTokens, completionTokens)
			DropSession(setup.requestID)
			return loopResult{earlyExit: true}
		}

		roundBuf := strings.Builder{}
		thinkingStarted := false
		thinkingStart := time.Now()

		result, streamErr := setup.stream(ctx, setup.baseURL, setup.apiKey, setup.model, messages, tools, setup.adapterOpts,
			func(chunk string, reasoning string, done bool) error {
				if reasoning != "" {
					if !thinkingStarted {
						thinkingStarted = true
						thinkingStart = time.Now()
					}
					if err := writeThinkingDeltaFrame(setup.w, reasoning); err != nil {
						return err
					}
				}
				if chunk != "" {
					if thinkingStarted {
						dur := int32(time.Since(thinkingStart).Milliseconds())
						_ = writeThinkingCompletedFrame(setup.w, dur)
						thinkingStarted = false
					}
					roundBuf.WriteString(chunk)
					assistantBuf.WriteString(chunk)
					if err := writeTextDeltaFrame(setup.w, chunk); err != nil {
						return err
					}
				}
				if done && thinkingStarted {
					dur := int32(time.Since(thinkingStart).Milliseconds())
					_ = writeThinkingCompletedFrame(setup.w, dur)
					thinkingStarted = false
				}
				return nil
			})

		// Track token usage.
		if result != nil && result.Usage != nil {
			promptTokens += int64(result.Usage.PromptTokens)
			completionTokens += int64(result.Usage.CompletionTokens)
			if delta := int64(result.Usage.PromptTokens + result.Usage.CompletionTokens); delta > 0 {
				d := delta
				if d > int64(^uint32(0)>>1) {
					d = int64(^uint32(0) >> 1)
				}
				_ = writeTokenDeltaFrame(setup.w, int32(d))
			}
		}
		lastResult = result

		// Handle stream errors (including context overflow → compaction).
		if streamErr != nil {
			if isContextOverflowError(streamErr) {
				logutil.Warn("context overflow", "error", streamErr)
				if art, ok := runCompaction(ctx, setup, messages, initialLen); ok {
					_ = art
					messages = buildMessageHistory(setup.sess)
					initialLen = len(messages)
					_ = writeTextDeltaFrame(setup.w, "\n[Context compressed. Continuing...]\n\n")
					continue
				}
			}
			logutil.Warn("stream error", "error", streamErr)
			persistPartialTurn(setup.sess, setup.requestID, setup.startedAt, messages, initialLen,
				setup.model, setup.baseURL, &assistantBuf, lastResult, promptTokens, completionTokens)
			DropSession(setup.requestID)
			return loopResult{earlyExit: true}
		}

		// Record assistant message.
		assistantMsg := textMessage("assistant", roundBuf.String())
		if len(result.ToolCalls) > 0 {
			for _, tc := range result.ToolCalls {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, openAIToolCallMsg{
					ID:       tc.ID,
					Type:     "function",
					Function: openAIToolCallFn{Name: tc.Name, Arguments: tc.Arguments},
				})
			}
		}
		messages = append(messages, assistantMsg)

		// No tool calls: done.
		if len(result.ToolCalls) == 0 {
			if err := writeTurnEndedFrame(setup.w); err != nil {
				logutil.Warn("write error", "error", err)
				persistPartialTurn(setup.sess, setup.requestID, setup.startedAt, messages, initialLen,
					setup.model, setup.baseURL, &assistantBuf, lastResult, promptTokens, completionTokens)
				DropSession(setup.requestID)
				return loopResult{earlyExit: true}
			}
			return loopResult{
				messages:         messages,
				initialLen:       initialLen,
				assistantBuf:     assistantBuf,
				lastResult:       lastResult,
				promptTokens:     promptTokens,
				completionTokens: completionTokens,
				round:            round,
			}
		}

		// Build checkpoint.
		var replay []byte
		if ctx := buildWorkspaceContext(setup.sess); ctx != "" {
			rep := map[string]string{"content": ctx, "role": "user"}
			if b, err := json.Marshal(rep); err == nil {
				replay = b
			}
		}
		_ = writeConversationCheckpoint(setup.w, replay)

		// Execute tools.
		messages = executeToolCalls(ctx, setup, messages, result, replay)
	}

	// Loop exited due to round/time cap.
	if lastResult != nil && len(lastResult.ToolCalls) > 0 {
		capWarning := "\n\n_[tool loop capped at " +
			itoa(setup.maxLoopRounds) + " rounds — task may be incomplete. Send a follow-up to continue.]_"
		_ = writeTextDeltaFrame(setup.w, capWarning)
		assistantBuf.WriteString(capWarning)
		_ = writeTurnEndedFrame(setup.w)
	}

	return loopResult{
		messages:         messages,
		initialLen:       initialLen,
		assistantBuf:     assistantBuf,
		lastResult:       lastResult,
		promptTokens:     promptTokens,
		completionTokens: completionTokens,
		round:            0,
		capped:           true,
	}
}

// executeToolCalls runs each tool call from the LLM response and appends
// results to the message history.
func executeToolCalls(
	ctx context.Context,
	setup setupResult,
	messages []openAIMessage,
	result *streamResult,
	replay []byte,
) []openAIMessage {
	for _, pc := range result.ToolCalls {
		// Handle local-only tools.
		if localResult, pill, interaction, handled := handleLocalTool(setup.sess, pc); handled {
			if pill != nil {
				_ = writeToolCallStarted(setup.w, pc.ID, pill)
			}
			if interaction != nil && pc.Name == "SwitchMode" {
				waitCh := registerInteractionWait(interaction.GetId())
				_ = writeInteractionQuery(setup.w, interaction)
				verdict := waitForInteractionResponse(waitCh, 45*time.Second)
				switch {
				case verdict == nil:
					localResult = `{"result":"timeout","note":"User did not respond within 45 seconds."}`
				default:
					if sm := verdict.GetSwitchModeRequestResponse(); sm != nil {
						if rej := sm.GetRejected(); rej != nil {
							reason := rej.GetReason()
							localResult = `{"result":"rejected","reason":` + strutil.JSONStringEscape(reason) + `}`
							setup.sess.Mode = 0
						} else {
							localResult = `{"result":"approved"}`
						}
					}
				}
			} else if interaction != nil {
				_ = writeInteractionQuery(setup.w, interaction)
			}
			if pill != nil {
				_ = writeToolCallCompleted(setup.w, pc.ID, pill)
			}
			messages = append(messages, toolResultMessage(pc.ID, pc.Name, localResult))
			continue
		}

		// Build and write the tool call.
		tc, buildErr := buildToolCallProto(setup.sess, pc)
		if buildErr != "" {
			messages = append(messages, toolResultMessage(pc.ID, pc.Name, `{"error":`+strutil.JSONStringEscape(buildErr)+`}`))
			continue
		}
		if tc == nil {
			messages = append(messages, toolResultMessage(pc.ID, pc.Name, `{"error": "tool `+pc.Name+` not implemented in CursorBridge yet"}`))
			continue
		}
		if err := writeToolCallStarted(setup.w, pc.ID, tc); err != nil {
			return messages
		}
		execID := newExecID(execToolName(pc.Name))
		seq := nextExecSeq()
		waitCh := registerToolWait(pc.ID)
		registerExecIDAlias(execID, seq, pc.ID)
		if err := writeExecRequest(setup.w, execID, seq, pc, tc); err != nil {
			return messages
		}

		// Wait for result with timeout.
		waitWindow := effectiveToolTimeout(setup.adapterOpts.ToolExecTimeoutSec, DefaultAgentToolTimeout)
		shellBackground := false
		if pc.Name == "Shell" {
			var sa struct {
				BlockUntilMs  int32 `json:"block_until_ms"`
				BlockUntilMs2 int32 `json:"blockUntilMs"`
			}
			_ = json.Unmarshal([]byte(pc.Arguments), &sa)
			bu := sa.BlockUntilMs
			if bu == 0 {
				bu = sa.BlockUntilMs2
			}
			if bu == 0 && strings.Contains(pc.Arguments, "block_until_ms") {
				shellBackground = true
				waitWindow = 3 * time.Second
			} else if bu > 0 && bu < 30000 {
				waitWindow = time.Duration(bu+2000) * time.Millisecond
			}
		}
		env := waitForToolResult(ctx, waitCh, waitWindow, setup.w.Broken)
		attachToolResultToProto(tc, pc.Name, env)

		if err := writeToolCallCompleted(setup.w, pc.ID, tc); err != nil {
			return messages
		}

		// Build result content.
		content := env.ResultJSON
		if env.Error != "" {
			switch {
			case shellBackground && strings.Contains(env.Error, "timed out"):
				content = `<shell-backgrounded>
Command was started in background mode and is still running.
To inspect progress, use the Read tool on a file under the terminals folder.
</shell-backgrounded>`
			case strings.Contains(env.Error, "timed out"):
				content = `<shell-incomplete>
The foreground wait window expired without a terminal event.
</shell-incomplete>`
			default:
				content = `{"error":` + strutil.JSONStringEscape(env.Error) + `}`
			}
		}
		if content == "" {
			content = `{"result":"ok"}`
		}

		// Cap large tool outputs.
		const maxToolResultBytes = 24 * 1024
		if len(content) > maxToolResultBytes {
			content = content[:maxToolResultBytes] + "\n…[tool output truncated]"
		}
		messages = append(messages, toolResultMessage(pc.ID, pc.Name, content))
		_ = writeConversationCheckpoint(setup.w, replay)
	}
	return messages
}

// runCompaction attempts emergency context compaction and returns (archive, true)
// on success, or (nil, false) on failure.
func runCompaction(
	ctx context.Context,
	setup setupResult,
	messages []openAIMessage,
	initialLen int,
) (*CompactionArchive, bool) {
	compState := getCompactionState(setup.deps, setup.sess.ConversationID)
	compState.mu.Lock()
	if compState.Compacting {
		compState.mu.Unlock()
		return nil, false
	}
	compState.Compacting = true
	compState.mu.Unlock()

	_ = writeSummaryStartedFrame(setup.w)
	hist := HistoryFor(setup.sess.ConversationID)
	recentKeep := defaultRecentTurnsToKeep
	if len(hist) <= recentKeep {
		recentKeep = len(hist) / 2
		if recentKeep < 1 {
			recentKeep = 1
		}
	}
	maxTokens := getModelContextTokenLimit(setup.adapterOpts, setup.model)
	summary, keptTurns, compactErr := compactTurns(ctx, hist, recentKeep, maxTokens, setup.stream, setup.baseURL, setup.apiKey, setup.model, setup.adapterOpts)

	compState.mu.Lock()
	defer compState.mu.Unlock()
	if compactErr != nil {
		compState.Compacting = false
		_ = writeSummaryCompletedFrame(setup.w, "Emergency compaction failed")
		return nil, false
	}
	archive := buildCompactionArchive(summary, hist, len(hist)-len(keptTurns))
	compState.Archives = append(compState.Archives, archive)
	compState.CurrentSummary = mergeSummaryWithExisting(compState.CurrentSummary, summary)
	compState.LastCompaction = time.Now()
	compState.Compacting = false

	_ = writeSummaryFrame(setup.w, summary)
	_ = writeSummaryCompletedFrame(setup.w, "Context compressed after overflow error — retrying")
	return &archive, true
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b strings.Builder
	for i > 0 {
		b.WriteByte(byte('0' + i%10))
		i /= 10
	}
	// Reverse the string.
	runes := []rune(b.String())
	for m, n := 0, len(runes)-1; m < n; m, n = m+1, n-1 {
		runes[m], runes[n] = runes[n], runes[m]
	}
	return string(runes)
}