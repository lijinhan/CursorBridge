package agent

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"cursorbridge/internal/codec"
	"cursorbridge/internal/debuglog"
	"cursorbridge/internal/strutil"
	"net/http"
	"strings"

	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"
	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

// Result is returned by the unary handlers (BidiAppend) so the MITM bridge
// can build a goproxy http.Response. Body is the marshalled response proto;
// Status is the HTTP status the bridge should set; ContentType is filled
// for non-error replies and left empty otherwise (bridge defaults to JSON).
type Result struct {
	Status      int
	ContentType string
	Body        []byte
}

// HandleBidiAppend parses the incoming BidiAppendRequest, decodes the inner
// AgentClientMessage, extracts the user message + model bindings, and stows
// them in the session store keyed by RequestId so the matching RunSSE call
// can pick them up. Returns an empty BidiAppendResponse — Cursor accepts a
// 0-byte body for unary success on this RPC.
func HandleBidiAppend(reqBody []byte, contentType string) Result {
	bidi := &aiserverv1.BidiAppendRequest{}
	if err := decodeUnary(reqBody, contentType, bidi); err != nil {
		return errResult(http.StatusBadRequest, "decode bidi append: "+err.Error())
	}
	if shouldRouteToBugBot(bidi) {
		return handleBugBotBidiAppendDecoded(bidi)
	}
	return handleAgentBidiAppendDecoded(bidi)
}

func handleAgentBidiAppendDecoded(bidi *aiserverv1.BidiAppendRequest) Result {
	if bidi == nil {
		return errResult(http.StatusBadRequest, "missing bidi append body")
	}

	requestID := ""
	if bidi.RequestId != nil {
		requestID = bidi.RequestId.RequestId
	}
	if requestID == "" {
		return errResult(http.StatusBadRequest, "missing request_id")
	}

	// Cursor stuffs the AgentClientMessage proto into BidiAppendRequest.Data
	// as hex (lower-case, no separator). Empty Data means a heartbeat / ack
	// frame — we just record the request_id with no extra state.
	sess := &Session{RequestID: requestID}
	if existing := GetSession(requestID); existing != nil {
		sess = existing
	}

	if bidi.Data != "" {
		raw, err := hex.DecodeString(strings.TrimSpace(bidi.Data))
		if err != nil {
			return errResult(http.StatusBadRequest, "bidi data not hex: "+err.Error())
		}
		acm := &agentv1.AgentClientMessage{}
		func() {
			defer func() {
				if r := recover(); r != nil {
					debuglog.Printf("[BIDI] proto.Unmarshal AgentClientMessage panicked: %v", r)
					err = fmt.Errorf("proto panic: %v", r)
				}
			}()
			err = proto.Unmarshal(raw, acm)
		}()
		if err != nil {
			return errResult(http.StatusBadRequest, "parse agent client message: "+err.Error())
		}
		extractIntoSession(sess, acm)
		// Route tool results back to the RunSSE goroutine waiting on them.
		// Cursor posts ExecClientMessage with the matching tool_call_id
		// (exec_id) once the IDE finishes running a tool.
		routeExecClientResult(acm)
		// Route interaction verdicts (user approved/rejected a SwitchMode
		// prompt, CreatePlan prompt, WebSearch prompt, etc.) back to the
		// RunSSE goroutine that's blocking on the user's decision.
		if ir := acm.GetInteractionResponse(); ir != nil {
			deliverInteractionResponse(ir)
		}
	}

	PutSession(sess)
	body, err := proto.Marshal(&aiserverv1.BidiAppendResponse{})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}

// HandleBugBotBidiAppend mirrors HandleBidiAppend for BugBot's parallel bidi
// transport. Cursor sends the initial StreamBugBotRequest and later tool
// results through the same BidiAppend envelope keyed by request_id.
func HandleBugBotBidiAppend(reqBody []byte, contentType string) Result {
	bidi := &aiserverv1.BidiAppendRequest{}
	if err := decodeUnary(reqBody, contentType, bidi); err != nil {
		return errResult(http.StatusBadRequest, "decode bugbot bidi append: "+err.Error())
	}
	return handleBugBotBidiAppendDecoded(bidi)
}

func handleBugBotBidiAppendDecoded(bidi *aiserverv1.BidiAppendRequest) Result {
	if bidi == nil {
		return errResult(http.StatusBadRequest, "missing bugbot bidi append body")
	}

	requestID := ""
	if bidi.RequestId != nil {
		requestID = bidi.RequestId.RequestId
	}
	if requestID == "" {
		return errResult(http.StatusBadRequest, "missing request_id")
	}

	sess := &Session{RequestID: requestID}
	if existing := GetSession(requestID); existing != nil {
		sess = existing
	}

	if bidi.Data != "" {
		raw, err := hex.DecodeString(strings.TrimSpace(bidi.Data))
		if err != nil {
			return errResult(http.StatusBadRequest, "bugbot bidi data not hex: "+err.Error())
		}
		acm := &aiserverv1.StreamBugBotAgenticClientMessage{}
		if err := proto.Unmarshal(raw, acm); err != nil {
			return errResult(http.StatusBadRequest, "parse bugbot client message: "+err.Error())
		}
		if start := acm.GetStart(); start != nil {
			sess.BugBotRequest = start
			sess.UserText = buildBugBotPrompt(start)
			if sess.ConversationID == "" {
				sess.ConversationID = "bugbot:" + requestID
			}
		}
		if execMsg := acm.GetExecClientMessage(); execMsg != nil {
			routeBugBotExecClientResult(execMsg)
		}
	}

	PutSession(sess)
	body, err := proto.Marshal(&aiserverv1.BidiAppendResponse{})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}

func errResult(status int, msg string) Result {
	body := []byte(`{"code":"invalid_argument","message":` + strutil.JSONString(msg) + `}`)
	return Result{Status: status, ContentType: "application/json", Body: body}
}

// extractIntoSession pulls the bits of AgentClientMessage we care about into
// the session: the user's text, the chosen model, the conversation
// state (so future iterations can reconstruct full chat history), and
// Cursor's original system prompt (from custom_system_prompt or
// root_prompt_messages_json) so the BYOK model can use it instead of
// our synthetic defaultSystemPrompt.
func extractIntoSession(sess *Session, acm *agentv1.AgentClientMessage) {
	if run := acm.GetRunRequest(); run != nil {
		sess.Run = run
		sess.ModelDetails = run.GetModelDetails()
		sess.State = run.GetConversationState()
		if cid := run.GetConversationId(); cid != "" {
			sess.ConversationID = cid
		}
		// Extract Cursor's original system prompt. Priority:
		//  1. AgentRunRequest.custom_system_prompt (explicit override)
		//  2. system-role messages in ConversationState.root_prompt_messages_json
		if csp := run.GetCustomSystemPrompt(); csp != "" {
			sess.CursorSystemPrompt = csp
			debuglog.Printf("[DEBUG] custom_system_prompt len=%d prefix=%s", len(csp), strutil.Truncate(csp, 100))
		} else {
			debuglog.Printf("[DEBUG] custom_system_prompt is empty")
		}
		if action := run.GetAction(); action != nil {
			sess.Action = action
			if uma := action.GetUserMessageAction(); uma != nil {
				if msg := uma.GetUserMessage(); msg != nil {
					if msg.GetText() != "" {
						sess.UserText = msg.GetText()
					}
					if m := msg.GetMode(); m != agentv1.AgentMode_AGENT_MODE_UNSPECIFIED {
						sess.Mode = m
					}
				}
			} else if epa := action.GetExecutePlanAction(); epa != nil {
				var planText strings.Builder
				planText.WriteString("Execute the following plan:")
				if plan := epa.GetPlan(); plan != nil && plan.GetPlan() != "" {
					planText.WriteByte('\n')
					planText.WriteString(plan.GetPlan())
				}
				if pfc := epa.GetPlanFileContent(); pfc != "" {
					planText.WriteByte('\n')
					planText.WriteByte('\n')
					planText.WriteString("Plan file content:")
					planText.WriteByte('\n')
				}
				if pfu := epa.GetPlanFileUri(); pfu != "" {
					planText.WriteByte('\n')
					planText.WriteByte('\n')
					planText.WriteString("Plan file URI: ")
					planText.WriteString(pfu)
				}
				sess.UserText = planText.String()
				if em := epa.GetExecutionMode(); em != agentv1.AgentMode_AGENT_MODE_UNSPECIFIED {
					sess.Mode = em
				} else {
					sess.Mode = agentv1.AgentMode_AGENT_MODE_AGENT
				}
				debuglog.Printf("[BIDI] ExecutePlanAction: userTextLen=%d mode=%v", len(sess.UserText), sess.Mode)
			} else if spa := action.GetStartPlanAction(); spa != nil {
				if msg := spa.GetUserMessage(); msg != nil {
					if msg.GetText() != "" {
						sess.UserText = msg.GetText()
					}
					if m := msg.GetMode(); m != agentv1.AgentMode_AGENT_MODE_UNSPECIFIED {
						sess.Mode = m
					} else {
						sess.Mode = agentv1.AgentMode_AGENT_MODE_PLAN
					}
				}
				debuglog.Printf("[BIDI] StartPlanAction: userTextLen=%d mode=%v", len(sess.UserText), sess.Mode)
			} else if ra := action.GetResumeAction(); ra != nil {
				debuglog.Printf("[BIDI] ResumeAction: no new UserText, mode stays %v", sess.Mode)
			} else {
				debuglog.Printf("[BIDI] unhandled action type: %T", action)
			}
		}
		// If custom_system_prompt was empty, try extracting system-role content
		// from root_prompt_messages_json. These are JSON blobs that Cursor sends
		// as carry-forward context; some carry role="system" with the main prompt.
		if sess.CursorSystemPrompt == "" && sess.State != nil {
			blobs := sess.State.GetRootPromptMessagesJson()
			debuglog.Printf("[DEBUG] root_prompt_messages_json count=%d", len(blobs))
			for i, b := range blobs {
				debuglog.Printf("[DEBUG] root_prompt_messages_json[%d] len=%d prefix=%s", i, len(b), strutil.Truncate(string(b), 200))
			}
			sess.CursorSystemPrompt = extractSystemPromptFromRootMessages(sess.State)
			if sess.CursorSystemPrompt != "" {
				debuglog.Printf("[DEBUG] extracted system prompt from root_prompt_messages_json, len=%d", len(sess.CursorSystemPrompt))
			} else {
				debuglog.Printf("[DEBUG] no system-role messages in root_prompt_messages_json")
			}
		}
	}
	if pre := acm.GetPrewarmRequest(); pre != nil && sess.ModelDetails == nil {
		sess.ModelDetails = pre.GetModelDetails()
		if sess.CursorSystemPrompt == "" {
			if csp := pre.GetCustomSystemPrompt(); csp != "" {
				sess.CursorSystemPrompt = csp
			}
		}
	}
}

// extractSystemPromptFromRootMessages scans ConversationStateStructure's
// root_prompt_messages_json for system-role messages and concatenates their
// content. Returns empty string when no system-role messages are found —
// in that case the caller falls back to defaultSystemPrompt.
func extractSystemPromptFromRootMessages(state *agentv1.ConversationStateStructure) string {
	if state == nil {
		return ""
	}
	var parts []string
	for _, blob := range state.GetRootPromptMessagesJson() {
		if len(blob) == 0 {
			continue
		}
		var msg struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(blob, &msg); err != nil {
			continue
		}
		if msg.Role == "system" && msg.Content != "" {
			parts = append(parts, msg.Content)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

// truncate returns the first n chars of s, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func buildBugBotPrompt(req *aiserverv1.StreamBugBotRequest) string {
	if req == nil {
		return "Review the provided git diff and find issues."
	}
	var b strings.Builder
	b.WriteString("Review the provided git diff and find concrete bugs, regressions, or correctness issues. ")
	b.WriteString("Focus on actionable findings with file and line references.\n\n")
	if instr := strings.TrimSpace(req.GetUserInstructions()); instr != "" {
		b.WriteString("User instructions:\n")
		b.WriteString(instr)
		b.WriteString("\n\n")
	}
	if guide := strings.TrimSpace(req.GetBugDetectionGuidelines()); guide != "" {
		b.WriteString("Bug detection guidelines:\n")
		b.WriteString(guide)
		b.WriteString("\n\n")
	}
	b.WriteString("Git diff:\n")
	b.WriteString(renderBugBotDiff(req.GetGitDiff()))
	if files := req.GetContextFiles(); len(files) > 0 {
		b.WriteString("\n\nContext files:\n")
		for _, f := range files {
			if f == nil {
				continue
			}
			if name := strings.TrimSpace(f.GetRelativeWorkspacePath()); name != "" {
				b.WriteString("\nFile: ")
				b.WriteString(name)
				b.WriteString("\n")
			}
			if contents := strings.TrimSpace(f.GetContents()); contents != "" {
				b.WriteString(contents)
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

func renderBugBotDiff(diff *aiserverv1.GitDiff) string {
	if diff == nil || len(diff.GetDiffs()) == 0 {
		return "<empty diff>"
	}
	var b strings.Builder
	for i, fd := range diff.GetDiffs() {
		if fd == nil {
			continue
		}
		if i > 0 {
			b.WriteString("\n\n")
		}
		from := fd.GetFrom()
		to := fd.GetTo()
		if from == "" {
			from = "/dev/null"
		}
		if to == "" {
			to = "/dev/null"
		}
		b.WriteString("diff --git a/")
		b.WriteString(from)
		b.WriteString(" b/")
		b.WriteString(to)
		for _, ch := range fd.GetChunks() {
			if ch == nil {
				continue
			}
			b.WriteString("\n")
			b.WriteString(ch.GetContent())
		}
	}
	if b.Len() == 0 {
		return "<empty diff>"
	}
	return b.String()
}

func shouldRouteToBugBot(bidi *aiserverv1.BidiAppendRequest) bool {
	if bidi == nil || bidi.GetData() == "" {
		return false
	}
	requestID := ""
	if bidi.RequestId != nil {
		requestID = bidi.RequestId.GetRequestId()
	}
	if requestID != "" {
		if sess := GetSession(requestID); sess != nil && sess.BugBotRequest != nil {
			return true
		}
	}
	raw, err := hex.DecodeString(strings.TrimSpace(bidi.GetData()))
	if err != nil {
		return false
	}
	// The BidiAppend data is usually an AgentClientMessage, not a BugBot
	// message. Protobuf's oneof field coder can panic when the wire format
	// doesn't match the target message type, so we recover here to prevent
	// a single mis-routed request from crashing the entire process.
	acm := &aiserverv1.StreamBugBotAgenticClientMessage{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				debuglog.Printf("[BIDI] shouldRouteToBugBot: proto.Unmarshal panicked: %v", r)
				err = fmt.Errorf("proto panic: %v", r)
			}
		}()
		err = proto.Unmarshal(raw, acm)
	}()
	if err != nil {
		return false
	}
	return acm.GetStart() != nil
}

// ---- Connect unary codec helpers ----

// decodeUnary unmarshals a unary Connect request body into msg, transparently
// stripping the optional 5-byte envelope and gzip layer that connect-go
// emits when Content-Type is "application/connect+proto".
func decodeUnary(body []byte, contentType string, msg proto.Message) error {
	payload := body
	if isConnectEnvelope(contentType) && len(payload) >= 5 {
		flags := payload[0]
		length := int(binary.BigEndian.Uint32(payload[1:5]))
		if length >= 0 && len(payload) >= 5+length {
			payload = payload[5 : 5+length]
			if flags&flagCompressed != 0 {
				if up, gerr := codec.Gunzip(payload); gerr == nil {
					payload = up
				} else {
					return gerr
				}
			}
		}
	}
	return proto.Unmarshal(payload, msg)
}

func isConnectEnvelope(contentType string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(contentType)), "connect")
}

