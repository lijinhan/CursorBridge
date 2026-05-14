package agent

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// finalizeTurn handles post-loop cleanup: cap warnings, persistence, session drop.
func finalizeTurn(setup setupResult, res loopResult) {
	// If the loop exited due to round/time cap with pending tool calls,
	// emit a warning and TurnEnded instead of error.
	if res.capped && res.lastResult != nil && len(res.lastResult.ToolCalls) > 0 {
		capWarning := "\n\n_[tool loop capped at " + strconv.Itoa(setup.maxLoopRounds) +
			" rounds — task may be incomplete. Send a follow-up message to continue.]_"
		_ = writeTextDeltaFrame(setup.w, capWarning)
		res.assistantBuf.WriteString(capWarning)
		_ = writeTurnEndedFrame(setup.w)
	}

	// Persist the completed turn.
	var art *turnArtifacts
	if res.lastResult != nil {
		var tcNames []string
		for _, tc := range res.lastResult.ToolCalls {
			tcNames = append(tcNames, tc.Name+"#"+tc.ID)
		}
		billable := res.promptTokens + res.completionTokens
		startedAt := time.Unix(setup.startedAt, 0)
		summary := map[string]any{
			"request_id":        setup.requestID,
			"started_at":        startedAt.UTC().Format(time.RFC3339Nano),
			"finished_at":       time.Now().UTC().Format(time.RFC3339Nano),
			"duration_ms":       time.Since(startedAt).Milliseconds(),
			"model":             setup.model,
			"requested_model":   requestedModelForSession(setup.sess),
			"provider":          providerFromURL(setup.baseURL),
			"output_len":        res.assistantBuf.Len(),
			"message_count":     len(res.messages),
			"finish_reason":     res.lastResult.FinishReason,
			"tool_calls":        tcNames,
			"prompt_tokens":     res.promptTokens,
			"completion_tokens": res.completionTokens,
			"total_tokens":      billable,
		}
		summaryBytes, _ := json.MarshalIndent(summary, "", "  ")
		art = &turnArtifacts{
			RequestBody: res.lastResult.RequestBody,
			SSEJSONL:    res.lastResult.SSERaw,
			SummaryJSON: summaryBytes,
			TotalTokens: int(billable),
		}
	}

	// Build assistantText from buffer. If assistant produced only tool calls
	// (zero visible text), synthesize a placeholder.
	assistantText := res.assistantBuf.String()
	if assistantText == "" && res.lastResult != nil && len(res.lastResult.ToolCalls) > 0 {
		var names []string
		for _, tc := range res.lastResult.ToolCalls {
			names = append(names, tc.Name)
		}
		assistantText = "[tool-only turn: " + strings.Join(names, ", ") + "]"
	}

	// Collect messages this turn appended for history replay.
	var storedMessages []StoredMessage
	if len(res.messages) > res.initialLen {
		storedMessages = make([]StoredMessage, 0, len(res.messages)-res.initialLen)
		for _, m := range res.messages[res.initialLen:] {
			storedMessages = append(storedMessages, openAIToStored(m))
		}
	}

	RecordTurn(setup.sess.ConversationID, setup.requestID, setup.sess.UserText,
		assistantText, modeString(setup.sess.Mode), art, storedMessages)
	_ = writeEndStream(setup.w)
	DropSession(setup.requestID)
}