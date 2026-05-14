package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"cursorbridge/internal/logutil"
	"strings"
	"time"

	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"
	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"
)

// buildToolCallProto maps a PendingToolCall (the model's function-call
// intent) into agent.v1.ToolCall. Handles MCP tool re-routing, then
// dispatches to the toolBuilder registry. Returns nil for unknown tools;
// errorMsg is set for build failures (caller should skip exec).
func buildToolCallProto(sess *Session, pc PendingToolCall) (*agentv1.ToolCall, string) {
	// Handle mcp_<idx>__<slug> re-routing before the registry dispatch.
	if strings.HasPrefix(pc.Name, "mcp_") {
		return buildMCPToolCall(sess, pc)
	}
	// Dispatch registered tools.
	return dispatchToolBuilder(sess, pc, toolBuilderRegistry)
}

// writeToolCallStarted emits the UI-facing "started" frame Cursor draws as
// a spinning tool pill while the real work is executing. call_id must
// match the tool_call_id the model picked; Cursor uses it to correlate
// the following Completed frame.
func writeToolCallStarted(w io.Writer, callID string, tc *agentv1.ToolCall) error {
	msg := &agentv1.AgentServerMessage{
		Message: &agentv1.AgentServerMessage_InteractionUpdate{
			InteractionUpdate: &agentv1.InteractionUpdate{
				Message: &agentv1.InteractionUpdate_ToolCallStarted{
					ToolCallStarted: &agentv1.ToolCallStartedUpdate{
						CallId:   callID,
						ToolCall: tc,
					},
				},
			},
		},
	}
	return writeAgentServerMessage(w, msg)
}

// writeToolCallCompleted emits the paired completion frame once Cursor
// finished running the tool and fed us a result.
func writeToolCallCompleted(w io.Writer, callID string, tc *agentv1.ToolCall) error {
	msg := &agentv1.AgentServerMessage{
		Message: &agentv1.AgentServerMessage_InteractionUpdate{
			InteractionUpdate: &agentv1.InteractionUpdate{
				Message: &agentv1.InteractionUpdate_ToolCallCompleted{
					ToolCallCompleted: &agentv1.ToolCallCompletedUpdate{
						CallId:   callID,
						ToolCall: tc,
					},
				},
			},
		},
	}
	return writeAgentServerMessage(w, msg)
}

// writeExecRequest fires the ExecServerMessage that actually tells Cursor's
// IDE to run the tool. The real work happens inside Cursor; we just hand
// over the args and then block on BidiAppend for the ExecClientMessage
// result carrying stdout/stderr/exit/etc.
//
// Wire format was reverse-engineered from the working app (RunSSE capture
// frame 5): shell runs use field 14 (shell_stream_args, uses ShellArgs
// type) — NOT field 2 (shell_args) — Cursor's IDE client ignores the
// deprecated field 2 path entirely. The exec_id must follow the
// "exec-<tool>-<nanos>" naming; the assistant's OpenAI tool_call_id goes
// into ShellArgs.ToolCallId so the completion frame can correlate.
// execSeq is the monotonic id field Cursor uses to order concurrent exec
// requests within a turn.
func writeExecRequest(w io.Writer, execID string, execSeq uint32, pc PendingToolCall, tc *agentv1.ToolCall) error {
	esm := &agentv1.ExecServerMessage{Id: execSeq, ExecId: execID}
	switch inner := tc.Tool.(type) {
	case *agentv1.ToolCall_ShellToolCall:
		esm.Message = &agentv1.ExecServerMessage_ShellStreamArgs{ShellStreamArgs: inner.ShellToolCall.Args}
	case *agentv1.ToolCall_ReadToolCall:
		esm.Message = &agentv1.ExecServerMessage_ReadArgs{ReadArgs: &agentv1.ReadArgs{
			Path:       inner.ReadToolCall.Args.Path,
			ToolCallId: pc.ID,
		}}
	case *agentv1.ToolCall_DeleteToolCall:
		esm.Message = &agentv1.ExecServerMessage_DeleteArgs{DeleteArgs: inner.DeleteToolCall.Args}
	case *agentv1.ToolCall_EditToolCall:
		// Write/StrReplace tools map to EditToolCall. The exec channel for
		// edits lives on ExecServerMessage.write_args (field 3).
		args := &agentv1.WriteArgs{ToolCallId: pc.ID, ReturnFileContentAfterWrite: true}
		if a := inner.EditToolCall.Args; a != nil {
			args.Path = a.Path
			if a.StreamContent != nil {
				args.FileText = *a.StreamContent
			}
		}
		esm.Message = &agentv1.ExecServerMessage_WriteArgs{WriteArgs: args}
	case *agentv1.ToolCall_GlobToolCall:
		dir := "."
		if inner.GlobToolCall.Args.TargetDirectory != nil {
			dir = *inner.GlobToolCall.Args.TargetDirectory
		}
		esm.Message = &agentv1.ExecServerMessage_LsArgs{LsArgs: &agentv1.LsArgs{
			Path:       dir,
			ToolCallId: pc.ID,
		}}
	case *agentv1.ToolCall_GrepToolCall:
		esm.Message = &agentv1.ExecServerMessage_GrepArgs{GrepArgs: inner.GrepToolCall.Args}
	case *agentv1.ToolCall_McpToolCall:
		esm.Message = &agentv1.ExecServerMessage_McpArgs{McpArgs: inner.McpToolCall.Args}
	case *agentv1.ToolCall_ReadMcpResourceToolCall:
		esm.Message = &agentv1.ExecServerMessage_ReadMcpResourceExecArgs{
			ReadMcpResourceExecArgs: inner.ReadMcpResourceToolCall.Args,
		}
	case *agentv1.ToolCall_ListMcpResourcesToolCall:
		esm.Message = &agentv1.ExecServerMessage_ListMcpResourcesExecArgs{
			ListMcpResourcesExecArgs: inner.ListMcpResourcesToolCall.Args,
		}
	case *agentv1.ToolCall_ReadLintsToolCall:
		path := ""
		if args := inner.ReadLintsToolCall.Args; args != nil && len(args.Paths) > 0 {
			path = args.Paths[0]
		}
		esm.Message = &agentv1.ExecServerMessage_DiagnosticsArgs{DiagnosticsArgs: &agentv1.DiagnosticsArgs{
			Path:       path,
			ToolCallId: pc.ID,
		}}
	default:
		return fmt.Errorf("unsupported exec tool %T", inner)
	}
	msg := &agentv1.AgentServerMessage{
		Message: &agentv1.AgentServerMessage_ExecServerMessage{
			ExecServerMessage: esm,
		},
	}
	return writeAgentServerMessage(w, msg)
}

// newExecID mints an exec_id in the format Cursor's IDE client recognises.
// Working-app captures consistently show "exec-<tool>-<nanoseconds>" — the
// tool prefix ("shell", "write", "read", etc.) helps Cursor route the
// completion callback to the right tool pill.
func newExecID(tool string) string {
	return fmt.Sprintf("exec-%s-%d", tool, time.Now().UnixNano())
}

// routeExecClientResult dispatches an incoming ExecClientMessage to the
// RunSSE goroutine waiting on its tool result. Cursor streams ShellStream
// events across many BidiAppend calls (Start -> Stdout chunks -> Exit);
// for shells we accumulate until the Exit event lands and only then
// deliver a composite result. Non-shell results (Write, Read, Ls, Grep,
// Delete) arrive as one complete message, so we deliver them immediately.
func routeExecClientResult(acm *agentv1.AgentClientMessage) {
	ecm := acm.GetExecClientMessage()
	if ecm == nil {
		return
	}
	seq := ecm.GetId()
	DefaultDeps.PendingMu.Lock()
	callID := DefaultDeps.SeqAlias[seq].callID
	DefaultDeps.PendingMu.Unlock()
	// Only log non-shell frames at entry; shell frames are logged at
	// Start/Exit milestones inside the shell branch to avoid flooding
	// the log with hundreds of Stdout/Stderr lines per command.
	isShell := ecm.GetShellStream() != nil
	if !isShell {
		logutil.Debug("tool_exec: routeExecClientResult", "seq", seq, "execID", ecm.GetExecId(), "callID", callID,
			"hasReadResult", ecm.GetReadResult() != nil,
			"hasLsResult", ecm.GetLsResult() != nil,
			"hasGrepResult", ecm.GetGrepResult() != nil,
			"hasWriteResult", ecm.GetWriteResult() != nil,
			"hasDeleteResult", ecm.GetDeleteResult() != nil,
			"hasMcpResult", ecm.GetMcpResult() != nil)
	}

	// Shell streaming: accumulate chunks, deliver on Exit.
	if ss := ecm.GetShellStream(); ss != nil {
		DefaultDeps.PendingMu.Lock()
		state := DefaultDeps.ShellAccum[seq]
		if state == nil {
			state = &shellAccumState{}
			DefaultDeps.ShellAccum[seq] = state
		}
		DefaultDeps.PendingMu.Unlock()

		// If this seq already exited, skip further frames — Cursor sometimes
		// sends Stdout/Stderr chunks after the Exit event, and re-processing
		// them just produces NO WAITER noise.
		if state.Exited {
			return
		}

		switch evt := ss.GetEvent().(type) {
		case *agentv1.ShellStream_Start:
			state.Started = true
			logutil.Debug("tool_exec: ShellStream Start", "seq", seq, "callID", callID)
			publishInteractionUpdate(&aiserverv1.InteractionUpdate{Message: &aiserverv1.InteractionUpdate_ShellOutputDelta{ShellOutputDelta: &aiserverv1.ShellOutputDeltaUpdate{Event: &aiserverv1.ShellOutputDeltaUpdate_Start{Start: &aiserverv1.ShellStreamStart{}}}}})
		case *agentv1.ShellStream_Stdout:
			state.Stdout = append(state.Stdout, []byte(evt.Stdout.GetData())...)
			publishInteractionUpdate(&aiserverv1.InteractionUpdate{Message: &aiserverv1.InteractionUpdate_ShellOutputDelta{ShellOutputDelta: &aiserverv1.ShellOutputDeltaUpdate{Event: &aiserverv1.ShellOutputDeltaUpdate_Stdout{Stdout: &aiserverv1.ShellStreamStdout{Data: evt.Stdout.GetData()}}}}})
		case *agentv1.ShellStream_Stderr:
			state.Stderr = append(state.Stderr, []byte(evt.Stderr.GetData())...)
			publishInteractionUpdate(&aiserverv1.InteractionUpdate{Message: &aiserverv1.InteractionUpdate_ShellOutputDelta{ShellOutputDelta: &aiserverv1.ShellOutputDeltaUpdate{Event: &aiserverv1.ShellOutputDeltaUpdate_Stderr{Stderr: &aiserverv1.ShellStreamStderr{Data: evt.Stderr.GetData()}}}}})
		case *agentv1.ShellStream_Exit:
			state.Exited = true
			state.ExitCode = evt.Exit.GetCode()
			state.Cwd = evt.Exit.GetCwd()
			logutil.Debug("tool_exec: ShellStream Exit", "seq", seq, "callID", callID, "exitCode", state.ExitCode, "cwd", state.Cwd, "stdoutLen", len(state.Stdout), "stderrLen", len(state.Stderr))
			publishInteractionUpdate(&aiserverv1.InteractionUpdate{Message: &aiserverv1.InteractionUpdate_ShellOutputDelta{ShellOutputDelta: &aiserverv1.ShellOutputDeltaUpdate{Event: &aiserverv1.ShellOutputDeltaUpdate_Exit{Exit: &aiserverv1.ShellStreamExit{Code: evt.Exit.GetCode(), Cwd: evt.Exit.GetCwd()}}}}})
		}
		if state.Exited && callID != "" {
			body, _ := json.Marshal(map[string]any{
				"exit_code": state.ExitCode,
				"cwd":       state.Cwd,
				"stdout":    string(state.Stdout),
				"stderr":    string(state.Stderr),
			})
			finalState := *state
			DefaultDeps.PendingMu.Lock()
			delete(DefaultDeps.ShellAccum, seq)
			delete(DefaultDeps.SeqAlias, seq)
			DefaultDeps.PendingMu.Unlock()
			deliverToolResult(callID, &toolResultEnvelope{
				ResultJSON: string(body),
				ShellAccum: &finalState,
			})
		}
		return
	}

	// Non-shell result — marshal whole ExecClientMessage and deliver.
	body, err := json.Marshal(ecm)
	if err != nil {
		return
	}
	env := &toolResultEnvelope{ResultJSON: string(body), ExecClient: ecm}
	logutil.Debug("tool_exec: routeExecClientResult non-shell", "seq", seq, "execID", ecm.GetExecId(), "callID", callID, "resultLen", len(body),
		"hasReadResult", ecm.GetReadResult() != nil,
		"hasLsResult", ecm.GetLsResult() != nil,
		"hasGrepResult", ecm.GetGrepResult() != nil,
		"hasWriteResult", ecm.GetWriteResult() != nil,
		"hasDeleteResult", ecm.GetDeleteResult() != nil,
		"hasMcpResult", ecm.GetMcpResult() != nil)
	if id := ecm.GetExecId(); id != "" {
		deliverToolResult(id, env)
		return
	}
	if callID != "" {
		DefaultDeps.PendingMu.Lock()
		delete(DefaultDeps.SeqAlias, seq)
		DefaultDeps.PendingMu.Unlock()
		deliverToolResult(callID, env)
		return
	}
	// Legacy fallback: single pending waiter gets everything.
	DefaultDeps.PendingMu.Lock()
	var only string
	count := 0
	for id := range DefaultDeps.Pending {
		only = id
		count++
	}
	DefaultDeps.PendingMu.Unlock()
	logutil.Debug("tool_exec: routeExecClientResult fallback", "pendingCount", count, "only", only)
	if count == 1 {
		deliverToolResult(only, env)
	} else {
		logutil.Warn("tool_exec: DROPPING result — cannot resolve target", "pendingCount", count, "seq", seq, "execID", ecm.GetExecId(), "callID", callID)
	}
}