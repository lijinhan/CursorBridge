package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"cursorbridge/internal/debuglog"
	"strings"
	"time"

	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"
)

// execToolName extracts the lowercase tool slug from an OpenAI tool name
// ("Shell" -> "shell") for exec_id composition.
func execToolName(openAIName string) string {
	return strings.ToLower(openAIName)
}

// pendingResult is the channel a RunSSE goroutine blocks on while waiting
// for Cursor's IDE to execute a tool and BidiAppend the result back. We
// key by tool_call_id so concurrent tool calls within the same turn each
// get their own waiter.
type pendingResult struct {
	ch chan *toolResultEnvelope
}

// toolResultEnvelope bundles what BidiAppend hands us when Cursor finishes
// running a tool. ResultJSON is what we'll feed OpenAI as the "tool" role
// message content; ExecClient is the raw proto so we can attach the
// matching Result field onto the ToolCall for pill rendering.
// Error is set instead on failure.
type toolResultEnvelope struct {
	ResultJSON string
	ExecClient *agentv1.ExecClientMessage
	ShellAccum *shellAccumState // populated for Shell so UI render has the pieces
	Error      string
}


type aliasEntry struct {
	callID string
	createdAt time.Time
}

type interactionEntry struct {
	ch        chan *agentv1.InteractionResponse
	createdAt time.Time
}

const entryTTL = 5 * time.Minute

// Default tool execution timeouts used when ToolExecTimeoutSec is 0.
const (
	DefaultAgentToolTimeout     = 30 * time.Second
	DefaultBugBotToolTimeout    = 30 * time.Second
	DefaultBgComposerToolTimeout = 2 * time.Minute
)

// effectiveToolTimeout returns the configured timeout or the default.
func effectiveToolTimeout(configured int, fallback time.Duration) time.Duration {
	if configured > 0 {
		return time.Duration(configured) * time.Second
	}
	return fallback
}

func init() {
	go sweepStaleEntries()
}

func sweepStaleEntries() {
	for range DefaultDeps.sweepTicker(30 * time.Second) {
		cutoff := time.Now().Add(-entryTTL)
		DefaultDeps.PendingMu.Lock()
		for id, e := range DefaultDeps.ExecIDAlias {
			if e.createdAt.Before(cutoff) {
				delete(DefaultDeps.ExecIDAlias, id)
			}
		}
		for seq, e := range DefaultDeps.SeqAlias {
			if e.createdAt.Before(cutoff) {
				delete(DefaultDeps.SeqAlias, seq)
			}
		}
		for id, e := range DefaultDeps.PendingInteraction {
			if e.createdAt.Before(cutoff) {
				close(e.ch)
				delete(DefaultDeps.PendingInteraction, id)
			}
		}
		// Clean up shellAccum entries that never received an Exit event.
		for seq, st := range DefaultDeps.ShellAccum {
			if st.Exited {
				continue
			}
			if st.startedAt.Before(cutoff) {
				delete(DefaultDeps.ShellAccum, seq)
				delete(DefaultDeps.SeqAlias, seq)
			}
		}
		// Clean up pending results whose goroutine has already timed out.
		for id, pr := range DefaultDeps.Pending {
			select {
			case <-pr.ch:
				delete(DefaultDeps.Pending, id)
			default:
			}
		}
		DefaultDeps.PendingMu.Unlock()
	}
}

func registerInteractionWait(id uint32) chan *agentv1.InteractionResponse {
	ch := make(chan *agentv1.InteractionResponse, 1)
	DefaultDeps.PendingMu.Lock()
	DefaultDeps.PendingInteraction[id] = interactionEntry{ch: ch, createdAt: time.Now()}
	DefaultDeps.PendingMu.Unlock()
	return ch
}

func deliverInteractionResponse(resp *agentv1.InteractionResponse) {
	if resp == nil {
		return
	}
	id := resp.GetId()
	DefaultDeps.PendingMu.Lock()
	ie := DefaultDeps.PendingInteraction[id]
	delete(DefaultDeps.PendingInteraction, id)
	DefaultDeps.PendingMu.Unlock()
	if ie.ch != nil {
		select {
		case ie.ch <- resp:
		default:
		}
	}
}

// waitForInteractionResponse blocks until Cursor's IDE delivers the user's
// verdict on the InteractionQuery we emitted (approve / reject), or the
// timeout elapses. Returns nil on timeout.
func waitForInteractionResponse(ch <-chan *agentv1.InteractionResponse, timeout time.Duration) *agentv1.InteractionResponse {
	select {
	case r := <-ch:
		return r
	case <-time.After(timeout):
		return nil
	}
}

type shellAccumState struct {
	Stdout    []byte
	Stderr    []byte
	Started   bool
	Exited    bool
	ExitCode  uint32
	Cwd       string
	startedAt time.Time
}

// registerExecIDAlias remembers that a future BidiAppend with execID
// (string) OR seq (numeric ExecServerMessage.Id we emitted) should be
// routed to the waiter registered against toolCallID.
func registerExecIDAlias(execID string, seq uint32, toolCallID string) {
	DefaultDeps.PendingMu.Lock()
	now := time.Now()
	DefaultDeps.ExecIDAlias[execID] = aliasEntry{callID: toolCallID, createdAt: now}
	DefaultDeps.SeqAlias[seq] = aliasEntry{callID: toolCallID, createdAt: now}
	DefaultDeps.PendingMu.Unlock()
	debuglog.Printf("[TOOL-EXEC] registerExecIDAlias: execID=%s seq=%d toolCallID=%s", execID, seq, toolCallID)
}

func registerToolWait(toolCallID string) chan *toolResultEnvelope {
	ch := make(chan *toolResultEnvelope, 1)
	DefaultDeps.PendingMu.Lock()
	DefaultDeps.Pending[toolCallID] = &pendingResult{ch: ch}
	DefaultDeps.PendingMu.Unlock()
	return ch
}

func deliverToolResult(id string, env *toolResultEnvelope) {
	DefaultDeps.PendingMu.Lock()
	// Accept either the OpenAI tool_call_id or Cursor's exec_id alias.
	pr := DefaultDeps.Pending[id]
	if pr == nil {
		if ae, ok := DefaultDeps.ExecIDAlias[id]; ok {
			pr = DefaultDeps.Pending[ae.callID]
			debuglog.Printf("[TOOL-EXEC] deliverToolResult: id=%s resolved via execIDAlias to callID=%s, found=%v", id, ae.callID, pr != nil)
			delete(DefaultDeps.Pending, ae.callID)
			delete(DefaultDeps.ExecIDAlias, id)
		}
	} else {
		debuglog.Printf("[TOOL-EXEC] deliverToolResult: id=%s found directly in pending, resultLen=%d hasError=%v", id, len(env.ResultJSON), env.Error != "")
		delete(DefaultDeps.Pending, id)
	}
	DefaultDeps.PendingMu.Unlock()
	if pr != nil {
		select {
		case pr.ch <- env:
			debuglog.Printf("[TOOL-EXEC] deliverToolResult: delivered result to waiter id=%s", id)
		default:
			debuglog.Printf("[TOOL-EXEC] deliverToolResult: waiter id=%s channel full, dropping result", id)
		}
	} else {
		debuglog.Printf("[TOOL-EXEC] deliverToolResult: NO WAITER for id=%s — result dropped! pendingCount=%d", id, len(DefaultDeps.Pending))
	}
}

// handleLocalTool resolves tool calls that don't need to round-trip through
// Cursor's IDE — e.g. SwitchMode, CreatePlan — by mutating session state and
// feeding the result straight back to the model.
//
// pillProto (when non-nil) becomes a ToolCallStarted/ToolCallCompleted pair
// so the call shows up in the chat bubble.
//
// interaction (when non-nil) is fired as an AgentServerMessage.interaction_query
// — this is the ONLY channel that moves Cursor's UI affordances like the
// mode-selector picker. SwitchMode needs this; the pill alone doesn't flip
// the selector.
func handleLocalTool(sess *Session, pc PendingToolCall) (result string, pillProto *agentv1.ToolCall, interaction *agentv1.InteractionQuery, handled bool) {
	switch pc.Name {
	case "SwitchMode":
		var a struct {
			TargetModeId string `json:"target_mode_id"`
			Explanation  string `json:"explanation"`
		}
		if err := json.Unmarshal([]byte(pc.Arguments), &a); err != nil {
			return `{"error":"parse SwitchMode args: ` + jsonStringEscapeRaw(err.Error()) + `"}`, nil, nil, true
		}
		target := agentv1.AgentMode_AGENT_MODE_UNSPECIFIED
		switch strings.ToLower(strings.TrimSpace(a.TargetModeId)) {
		case "agent", "agent_mode_agent":
			target = agentv1.AgentMode_AGENT_MODE_AGENT
		case "ask", "agent_mode_ask":
			target = agentv1.AgentMode_AGENT_MODE_ASK
		case "plan", "agent_mode_plan":
			target = agentv1.AgentMode_AGENT_MODE_PLAN
		case "debug", "agent_mode_debug":
			target = agentv1.AgentMode_AGENT_MODE_DEBUG
		default:
			return `{"error":"unknown target_mode_id (expected: agent, ask, plan, debug)"}`, nil, nil, true
		}
		if sess != nil {
			sess.Mode = target
		}
		// Cursor's UI picker keys options by an internal id that doesn't
		// always match the enum name — confirmed by grepping Cursor's
		// workbench.desktop.main.js for picker definitions:
		//   { id:"agent", name:"Agent" }
		//   { id:"plan",  name:"Plan"  }
		//   { id:"debug", name:"Debug" }
		//   { id:"chat",  name:"Ask"   }  ← the "Ask" label ships as "chat"
		// Send the id, not the enum-derived shortLabel, so every mode
		// flips the picker correctly.
		pickerID := cursorPickerID(target)
		switchArgs := &agentv1.SwitchModeArgs{
			TargetModeId: pickerID,
			ToolCallId:   pc.ID,
		}
		if a.Explanation != "" {
			expl := a.Explanation
			switchArgs.Explanation = &expl
		}
		pill := &agentv1.ToolCall{
			Tool: &agentv1.ToolCall_SwitchModeToolCall{
				SwitchModeToolCall: &agentv1.SwitchModeToolCall{
					Args: switchArgs,
				},
			},
		}
		// The pill alone doesn't move Cursor's mode selector — only an
		// InteractionQuery with SwitchModeRequestQuery does. Fire it so the
		// picker flips and the IDE treats this like a user-approved switch.
		iq := &agentv1.InteractionQuery{
			Id: interactionQueryID(pc.ID),
			Query: &agentv1.InteractionQuery_SwitchModeRequestQuery{
				SwitchModeRequestQuery: &agentv1.SwitchModeRequestQuery{
					Args: switchArgs,
				},
			},
		}
		return fmt.Sprintf(`{"result":"ok","mode":%q,"note":"Mode switched. Continue the task in the new mode without re-asking the user."}`, a.TargetModeId), pill, iq, true
	case "CreatePlan":
		var a struct {
			Plan     string   `json:"plan"`
			Name     string   `json:"name"`
			Overview string   `json:"overview"`
			Todos    []string `json:"todos"`
		}
		if err := json.Unmarshal([]byte(pc.Arguments), &a); err != nil {
			return `{"error":"parse CreatePlan args: ` + jsonStringEscapeRaw(err.Error()) + `"}`, nil, nil, true
		}
		if sess == nil {
			return `{"error":"no session"}`, nil, nil, true
		}
		// Build the authoritative todo list. Store goes into the conversation-
		// scoped map (so next turn's session sees the same plan); session's
		// own copy is for the current turn's <active_plan> projection only.
		todos := make([]*TodoEntry, 0, len(a.Todos))
		protoTodos := make([]*agentv1.TodoItem, 0, len(a.Todos))
		for i, t := range a.Todos {
			content := strings.TrimSpace(t)
			id := fmt.Sprintf("t%d", i+1)
			todos = append(todos, &TodoEntry{ID: id, Content: content, Status: "pending"})
			protoTodos = append(protoTodos, &agentv1.TodoItem{
				Id:      id,
				Content: content,
				Status:  agentv1.TodoStatus_TODO_STATUS_PENDING,
			})
		}
		SavePlanState(sess.ConversationID, a.Name, a.Overview, todos)
		sess.PlanName = a.Name
		sess.PlanOverview = a.Overview
		sess.Todos = todos
		planArgs := &agentv1.CreatePlanArgs{
			Plan:     a.Plan,
			Name:     a.Name,
			Overview: a.Overview,
			Todos:    protoTodos,
		}
		pill := &agentv1.ToolCall{
			Tool: &agentv1.ToolCall_CreatePlanToolCall{
				CreatePlanToolCall: &agentv1.CreatePlanToolCall{
					Args: planArgs,
					Result: &agentv1.CreatePlanResult{
						Result: &agentv1.CreatePlanResult_Success{
							Success: &agentv1.CreatePlanSuccess{},
						},
					},
				},
			},
		}
		// Gate the InteractionQuery per-conversation: Cursor's native Plan
		// panel creates a fresh .plan.md file for every CreatePlanRequestQuery
		// it receives, so letting the model fire CreatePlan repeatedly within
		// one chat would spawn a dozen plan files and flood <open_files> on
		// the next turn. Keying the gate off conversation_id (not session
		// state) keeps it clean across reconnect-clones and continuation
		// rounds while still giving each new chat a fresh panel.
		var iq *agentv1.InteractionQuery
		if !PlanEmittedFor(sess.ConversationID) {
			iq = &agentv1.InteractionQuery{
				Id: interactionQueryID(pc.ID),
				Query: &agentv1.InteractionQuery_CreatePlanRequestQuery{
					CreatePlanRequestQuery: &agentv1.CreatePlanRequestQuery{
						Args:       planArgs,
						ToolCallId: pc.ID,
					},
				},
			}
			MarkPlanEmitted(sess.ConversationID)
		}
		body, _ := json.Marshal(map[string]any{
			"result": "ok",
			"name":   sess.PlanName,
			"todos":  renderTodosJSON(sess.Todos),
			"note":   "Plan created. Continue work; use UpdateTodo as items progress. The plan persists in this conversation — do NOT call CreatePlan again for the same task.",
		})
		return string(body), pill, iq, true
	case "AddTodo":
		var a struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(pc.Arguments), &a); err != nil {
			return `{"error":"parse AddTodo args: ` + jsonStringEscapeRaw(err.Error()) + `"}`, nil, nil, true
		}
		a.Content = strings.TrimSpace(a.Content)
		if a.Content == "" {
			return `{"error":"content required"}`, nil, nil, true
		}
		if sess == nil {
			return `{"error":"no session"}`, nil, nil, true
		}
		todos, ok := AppendTodo(sess.ConversationID, a.Content)
		if !ok {
			return `{"error":"no active plan — call CreatePlan first"}`, nil, nil, true
		}
		sess.Todos = todos
		body, _ := json.Marshal(map[string]any{
			"result": "ok",
			"id":     todos[len(todos)-1].ID,
			"todos":  renderTodosJSON(todos),
		})
		return string(body), updateTodosPill(todos), nil, true
	case "UpdateTodo":
		var a struct {
			ID      string `json:"id"`
			Content string `json:"content"`
			Status  string `json:"status"`
		}
		if err := json.Unmarshal([]byte(pc.Arguments), &a); err != nil {
			return `{"error":"parse UpdateTodo args: ` + jsonStringEscapeRaw(err.Error()) + `"}`, nil, nil, true
		}
		if sess == nil {
			return `{"error":"no session"}`, nil, nil, true
		}
		allowed := map[string]bool{"pending": true, "in_progress": true, "completed": true, "cancelled": true}
		if !allowed[a.Status] {
			return `{"error":"invalid status (expected: pending, in_progress, completed, cancelled)"}`, nil, nil, true
		}
		todos, ok := UpdateTodoStatus(sess.ConversationID, a.ID, a.Content, a.Status)
		if !ok {
			// Either no plan exists yet, or the lookup missed. Check which.
			if PlanStateFor(sess.ConversationID) == nil {
				return `{"error":"no active plan — call CreatePlan first"}`, nil, nil, true
			}
			return `{"error":"todo not found — pass id or a content prefix"}`, nil, nil, true
		}
		sess.Todos = todos
		// Surface the matched id in the response so the model can chain updates.
		matchedID := a.ID
		if matchedID == "" {
			for _, t := range todos {
				if t.Status == a.Status && (a.Content == "" || strings.HasPrefix(strings.ToLower(t.Content), strings.ToLower(a.Content))) {
					matchedID = t.ID
					break
				}
			}
		}
		body, _ := json.Marshal(map[string]any{
			"result": "ok",
			"id":     matchedID,
			"todos":  renderTodosJSON(todos),
		})
		return string(body), updateTodosPill(todos), nil, true
	case "AskQuestion":
		var a struct {
			Questions []struct {
				ID            string `json:"id"`
				Prompt        string `json:"prompt"`
				AllowMultiple bool   `json:"allow_multiple"`
				Options       []struct {
					ID    string `json:"id"`
					Label string `json:"label"`
				} `json:"options"`
			} `json:"questions"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal([]byte(pc.Arguments), &a); err != nil {
			return `{"error":"parse AskQuestion args: ` + jsonStringEscapeRaw(err.Error()) + `"}`, nil, nil, true
		}
		if len(a.Questions) == 0 {
			return `{"error":"at least one question required"}`, nil, nil, true
		}
		questions := make([]*agentv1.AskQuestionArgs_Question, 0, len(a.Questions))
		for _, q := range a.Questions {
			opts := make([]*agentv1.AskQuestionArgs_Option, 0, len(q.Options))
			for _, o := range q.Options {
				opts = append(opts, &agentv1.AskQuestionArgs_Option{
					Id:    o.ID,
					Label: o.Label,
				})
			}
			questions = append(questions, &agentv1.AskQuestionArgs_Question{
				Id:            q.ID,
				Prompt:        q.Prompt,
				AllowMultiple: q.AllowMultiple,
				Options:       opts,
			})
		}
		args := &agentv1.AskQuestionArgs{
			Title:     a.Title,
			Questions: questions,
		}
		pill := &agentv1.ToolCall{
			Tool: &agentv1.ToolCall_AskQuestionToolCall{
				AskQuestionToolCall: &agentv1.AskQuestionToolCall{
					Args: args,
				},
			},
		}
		iq := &agentv1.InteractionQuery{
			Id: interactionQueryID(pc.ID),
			Query: &agentv1.InteractionQuery_AskQuestionInteractionQuery{
				AskQuestionInteractionQuery: &agentv1.AskQuestionInteractionQuery{
					Args:        args,
					ToolCallId:  pc.ID,
				},
			},
		}
		return `{"result":"ok","note":"Waiting for user response."}`, pill, iq, true
	}
	return "", nil, nil, false
}

// cursorPickerID returns the Cursor UI picker id for a given AgentMode.
// These ids come from Cursor's bundled workbench JS, not from the proto
// enum: Ask ships as "chat" even though the enum name is AGENT_MODE_ASK.
func cursorPickerID(m agentv1.AgentMode) string {
	switch m {
	case agentv1.AgentMode_AGENT_MODE_AGENT:
		return "agent"
	case agentv1.AgentMode_AGENT_MODE_ASK:
		return "chat"
	case agentv1.AgentMode_AGENT_MODE_PLAN:
		return "plan"
	case agentv1.AgentMode_AGENT_MODE_DEBUG:
		return "debug"
	default:
		return strings.ToLower(strings.TrimPrefix(m.String(), "AGENT_MODE_"))
	}
}

// interactionQueryID derives a deterministic uint32 id from the tool_call_id
// so Cursor's InteractionResponse can correlate a reply back to the query
// we emitted. Full bit collapse is fine — we never send two concurrent
// queries with the same payload.
func interactionQueryID(toolCallID string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(toolCallID); i++ {
		h ^= uint32(toolCallID[i])
		h *= 16777619
	}
	if h == 0 {
		h = 1
	}
	return h
}

func renderTodosJSON(todos []*TodoEntry) []map[string]string {
	out := make([]map[string]string, 0, len(todos))
	for _, t := range todos {
		out = append(out, map[string]string{
			"id":      t.ID,
			"status":  t.Status,
			"content": t.Content,
		})
	}
	return out
}

// todosToProto converts the session's local TodoEntry slice into the proto
// TodoItem form Cursor's Plan panel renders.
func todosToProto(ts []*TodoEntry) []*agentv1.TodoItem {
	out := make([]*agentv1.TodoItem, 0, len(ts))
	for _, t := range ts {
		out = append(out, &agentv1.TodoItem{
			Id:      t.ID,
			Content: t.Content,
			Status:  todoStatusProto(t.Status),
		})
	}
	return out
}

func todoStatusProto(s string) agentv1.TodoStatus {
	switch s {
	case "in_progress":
		return agentv1.TodoStatus_TODO_STATUS_IN_PROGRESS
	case "completed":
		return agentv1.TodoStatus_TODO_STATUS_COMPLETED
	case "cancelled":
		return agentv1.TodoStatus_TODO_STATUS_CANCELLED
	default:
		return agentv1.TodoStatus_TODO_STATUS_PENDING
	}
}

// updateTodosPill builds an UpdateTodosToolCall pill carrying the CURRENT full
// todo list (Merge=false replaces the panel's state). AddTodo and UpdateTodo
// emit this after mutating sess.Todos so Cursor's native Plan panel refreshes
// its checkbox state live — without it the panel only reflects the initial
// CreatePlan snapshot and TODOs never appear to advance in the UI.
func updateTodosPill(todos []*TodoEntry) *agentv1.ToolCall {
	proto := todosToProto(todos)
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_UpdateTodosToolCall{
			UpdateTodosToolCall: &agentv1.UpdateTodosToolCall{
				Args: &agentv1.UpdateTodosArgs{Todos: proto, Merge: false},
				Result: &agentv1.UpdateTodosResult{
					Result: &agentv1.UpdateTodosResult_Success{
						Success: &agentv1.UpdateTodosSuccess{Todos: proto},
					},
				},
			},
		},
	}
}

func jsonStringEscapeRaw(s string) string {
	b, _ := json.Marshal(s)
	out := string(b)
	if len(out) >= 2 {
		return out[1 : len(out)-1]
	}
	return out
}
// waitForToolResult blocks until Cursor posts a matching tool_call_id
// result via BidiAppend (see bidi.go -> HandleBidiAppend). Returns a
// timeout envelope after the specified duration so a stuck IDE doesn't
// hang the server goroutine forever.
//
// The brokenFn callback (when non-nil) is polled every second; if it
// reports true the wait aborts immediately with a "client disconnected"
// error. This prevents the agent loop from blocking for the full timeout
// window after the SSE client has already gone away.
func waitForToolResult(ctx context.Context, ch chan *toolResultEnvelope, timeout time.Duration, brokenFn func() bool) *toolResultEnvelope {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			debuglog.Printf("[TOOL-EXEC] waitForToolResult: TIMED OUT after %s", timeout.String())
			return &toolResultEnvelope{Error: "tool execution timed out after " + timeout.String()}
		}
		iterWait := remaining
		if iterWait > time.Second {
			iterWait = time.Second
		}
		select {
		case env := <-ch:
			debuglog.Printf("[TOOL-EXEC] waitForToolResult: received result, resultLen=%d hasError=%v", len(env.ResultJSON), env.Error != "")
			return env
		case <-ctx.Done():
			debuglog.Printf("[TOOL-EXEC] waitForToolResult: context cancelled, aborting wait")
			return &toolResultEnvelope{Error: "tool execution cancelled: " + ctx.Err().Error()}
		case <-time.After(iterWait):
			if brokenFn != nil && brokenFn() {
				debuglog.Printf("[TOOL-EXEC] waitForToolResult: client disconnected, aborting wait")
				return &toolResultEnvelope{Error: "client disconnected during tool execution"}
			}
		}
	}
}
