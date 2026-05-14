package agent

import (
	"encoding/json"
	"cursorbridge/internal/debuglog"
	"cursorbridge/internal/strutil"

	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"

	"google.golang.org/protobuf/proto"
)

func wrapUserQuery(text string) string {
	return "<user_query>\n" + text + "\n</user_query>"
}

// buildMessageHistory walks the RunRequest's ConversationState.Turns and
// the current UserMessageAction to produce an OpenAI-style message list.
//
// Layout mirrors the working app's request artifacts:
//
//	system : agent persona
//	user   : workspace context block (<user_info>, etc.)
//	user   : <user_query>turn0</user_query>
//	assist : turn0 assistant reply
//	user   : <user_query>turn1</user_query>
//	assist : turn1 assistant reply
//	...
//	user   : <user_query>current message</user_query>
//
// Tool-call steps and thinking steps are skipped for the MVP — they don't
// matter for plain chat, and including them without round-tripping the
// matching tool_result frames would confuse the model.
func buildMessageHistory(sess *Session) []openAIMessage {
	out := make([]openAIMessage, 0, 8)
	out = append(out, textMessage("system", systemPromptFor(sess)))
	if ctx := buildWorkspaceContext(sess); ctx != "" {
		out = append(out, textMessage("user", ctx))
	}

	// Retrieve compaction state for this conversation — if a summary exists
	// from a prior compaction, inject it as a system-role context block that
	// replaces the old turns the summary already covers.
	compState := getCompactionState(DefaultDeps, sess.ConversationID)
	compState.mu.RLock()
	existingSummary := compState.CurrentSummary
	compState.mu.RUnlock()

	if hist := HistoryFor(sess.ConversationID); len(hist) > 0 {
		debuglog.Printf("[HISTORY] conversationID=%s found %d turns in local history", sess.ConversationID, len(hist))

		// If we have an existing compaction summary, inject it before the
		// turns that fall within the summary's window_tail (the turns the
		// summary covers are skipped; only turns after window_tail are
		// replayed in full). This replaces truncation with LLM-generated
		// compression — the model gets a concise summary of old context
		// instead of losing it entirely.
		compState.mu.RLock()
		windowTail := 0
		for _, arch := range compState.Archives {
			if arch.WindowTail > windowTail {
				windowTail = arch.WindowTail
			}
		}
		compState.mu.RUnlock()

		if existingSummary != "" && windowTail > 0 && windowTail < len(hist) {
			debuglog.Printf("[COMPACTION] conversationID=%s injecting summary (windowTail=%d, totalTurns=%d), skipping %d old turns", sess.ConversationID, windowTail, len(hist), windowTail)
			out = append(out, textMessage("user", "<conversation_summary>\nThe following is a summary of earlier conversation turns that were compressed to save context space. Use this as background context — it preserves key decisions, code changes, and unresolved issues from the preceding discussion:\n\n"+existingSummary+"\n</conversation_summary>"))
			// Only replay turns after the summary's window_tail
			for _, t := range hist[windowTail:] {
				out = appendTurnMessages(out, t)
			}
		} else {
			for _, t := range hist {
				out = appendTurnMessages(out, t)
			}
		}
	} else if sess.State != nil {
		debuglog.Printf("[HISTORY] conversationID=%s no local history, falling back to proto State.Turns (%d turns)", sess.ConversationID, len(sess.State.Turns))

		// Check for proto-level summary in ConversationStateStructure
		if summaryBytes := sess.State.GetSummary(); len(summaryBytes) > 0 {
			summaryText := string(summaryBytes)
			if summaryText != "" {
				debuglog.Printf("[COMPACTION] conversationID=%s injecting proto-level summary (len=%d)", sess.ConversationID, len(summaryText))
				out = append(out, textMessage("user", "<conversation_summary>\n"+summaryText+"\n</conversation_summary>"))
			}
		}

		// When proto state has summary_archives, extract the most recent
		// archive's summary and window_tail to skip already-summarized turns.
		protoWindowTail := 0
		if archives := sess.State.GetSummaryArchives(); len(archives) > 0 {
			for _, archBytes := range archives {
				arch := &agentv1.ConversationSummaryArchive{}
				if err := proto.Unmarshal(archBytes, arch); err == nil {
					if int(arch.GetWindowTail()) > protoWindowTail {
						protoWindowTail = int(arch.GetWindowTail())
					}
					if arch.GetSummary() != "" && existingSummary == "" {
						existingSummary = arch.GetSummary()
					}
				}
			}
		}
		if archiveBytes := sess.State.GetSummaryArchive(); len(archiveBytes) > 0 {
			arch := &agentv1.ConversationSummaryArchive{}
			if err := proto.Unmarshal(archiveBytes, arch); err == nil {
				if int(arch.GetWindowTail()) > protoWindowTail {
					protoWindowTail = int(arch.GetWindowTail())
				}
				if arch.GetSummary() != "" && existingSummary == "" {
					existingSummary = arch.GetSummary()
				}
			}
		}

		// Inject proto-level summary if we found one and haven't already
		if existingSummary != "" && len(sess.State.GetSummary()) == 0 {
			debuglog.Printf("[COMPACTION] conversationID=%s injecting archive-level summary (protoWindowTail=%d)", sess.ConversationID, protoWindowTail)
			out = append(out, textMessage("user", "<conversation_summary>\n"+existingSummary+"\n</conversation_summary>"))
		}

		for i, blob := range sess.State.Turns {
			if len(blob) == 0 {
				continue
			}
			// Skip turns covered by the proto summary archive
			if protoWindowTail > 0 && i < protoWindowTail {
				continue
			}
			turn := &agentv1.ConversationTurn{}
			if err := proto.Unmarshal(blob, turn); err != nil {
				continue
			}
			act := turn.GetAgentConversationTurn()
			if act == nil {
				continue
			}
			if msg := act.GetUserMessage(); msg != nil {
				if t := msg.GetText(); t != "" {
					out = append(out, textMessage("user", wrapUserQuery(t)))
				}
			}
			var assistant string
			for _, step := range act.GetSteps() {
				if a := step.GetAssistantMessage(); a != nil && a.GetText() != "" {
					assistant += a.GetText()
				}
			}
			if assistant != "" {
				out = append(out, textMessage("assistant", assistant))
			}
		}
	}

	if sess.UserText != "" {
		out = append(out, buildUserMessageWithImages(sess))
	}
	return out
}

// appendTurnMessages appends a single ConvTurn's messages to the output list.
func appendTurnMessages(out []openAIMessage, t *ConvTurn) []openAIMessage {
	if len(t.Messages) > 0 {
		if t.User != "" {
			out = append(out, textMessage("user", wrapUserQuery(t.User)))
		}
		for _, m := range t.Messages {
			out = append(out, storedToOpenAI(m))
		}
		return out
	}
	if t.User != "" {
		out = append(out, textMessage("user", wrapUserQuery(t.User)))
	}
	if t.Assistant != "" {
		out = append(out, textMessage("assistant", t.Assistant))
	}
	return out
}

// buildUserMessageWithImages constructs the final user message, attaching
// any images from SelectedContext as multipart content parts (base64 data URLs).
// Falls back to a plain text message when no images are present.
func buildUserMessageWithImages(sess *Session) openAIMessage {
	query := wrapUserQuery(sess.UserText)
	if sess.Action == nil {
		return textMessage("user", query)
	}
	uma := sess.Action.GetUserMessageAction()
	if uma == nil {
		return textMessage("user", query)
	}
	msg := uma.GetUserMessage()
	if msg == nil {
		return textMessage("user", query)
	}
	sc := msg.GetSelectedContext()
	if sc == nil || len(sc.GetSelectedImages()) == 0 {
		return textMessage("user", query)
	}
	parts := []openAIContentPart{{Type: "text", Text: query}}
	for _, img := range sc.GetSelectedImages() {
		var data []byte
		mime := img.GetMimeType()
		if blob := img.GetBlobIdWithData(); blob != nil {
			data = blob.GetData()
		}
		if len(data) == 0 {
			data = img.GetData()
		}
		if len(data) == 0 {
			continue
		}
		if mime == "" {
			mime = "image/png"
		}
		b64 := strutil.Base64Encode(data)
		parts = append(parts, openAIContentPart{
			Type:     "image_url",
			ImageURL: &openAIImageURL{URL: "data:" + mime + ";base64," + b64},
		})
	}
	if len(parts) == 1 {
		return textMessage("user", query)
	}
	return multipartMessage("user", parts)
}

// storedToOpenAI converts a disk-persisted message back into the live
// openAIMessage form the stream request builder expects. Used by
// buildMessageHistory to replay tool-heavy turns verbatim instead of losing
// their intermediate tool_calls + tool results to the plain-text fallback.
func storedToOpenAI(m StoredMessage) openAIMessage {
	out := openAIMessage{
		Role:       m.Role,
		ToolCallID: m.ToolCallID,
		Name:       m.Name,
	}
	if m.Content != "" {
		raw, _ := json.Marshal(m.Content)
		out.Content = raw
	}
	if len(m.ToolCalls) > 0 {
		out.ToolCalls = make([]openAIToolCallMsg, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, openAIToolCallMsg{
				ID:       tc.ID,
				Type:     "function",
				Function: openAIToolCallFn{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
	}
	return out
}

// openAIToStored is the inverse of storedToOpenAI — runsse.go calls it after a
// turn finishes so the disk record preserves whatever live messages the loop
// accumulated (assistant-with-tool_calls + tool-role results). Content is
// unwrapped from json.RawMessage back to a plain string for compact storage.
func openAIToStored(m openAIMessage) StoredMessage {
	out := StoredMessage{
		Role:       m.Role,
		ToolCallID: m.ToolCallID,
		Name:       m.Name,
	}
	if len(m.Content) > 0 {
		var s string
		if err := json.Unmarshal(m.Content, &s); err == nil {
			out.Content = s
		} else {
			// Content could be a multipart (image) array — keep raw JSON.
			out.Content = string(m.Content)
		}
	}
	if len(m.ToolCalls) > 0 {
		out.ToolCalls = make([]StoredToolCall, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, StoredToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}
	return out
}
