package agent

import (
	"encoding/json"

	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"
)

// toolBuilder builds a Cursor Agent ToolCall proto from a PendingToolCall.
// Returns (tool, errorMsg). errorMsg empty means success with zero-value tool;
// errorMsg non-empty means build failed with that message; tool==nil with
// errorMsg=="" means the tool name was not recognised.
type toolBuilder func(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string)

// toolBuilderWithSess is like toolBuilder but also receives the session,
// needed for MCP tool re-routing.
type toolBuilderWithSess func(sess *Session, pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string)

// toolBuilderEntry pairs a tool name with its builder. nil builder means the
// name is handled by buildToolCallProto's default case (returns "","").
type toolBuilderEntry struct {
	name    string
	builder toolBuilder
}

// dispatchToolBuilder looks up pc.Name in the registry and calls its builder.
// Returns ("", "") for unknown names (default case).
func dispatchToolBuilder(sess *Session, pc PendingToolCall, reg []toolBuilderEntry) (*agentv1.ToolCall, string) {
	for _, e := range reg {
		if e.name == pc.Name {
			return e.builder(pc)
		}
	}
	// mcp_ tools need the session for re-routing; handled before the switch.
	return nil, ""
}

// toolBuilderRegistry lists all known tool builders in the order they appear
// in the original switch. The first matching name wins.
var toolBuilderRegistry = []toolBuilderEntry{
	{"CallMcpTool", buildCallMcpToolCall},
	{"FetchMcpResource", buildFetchMcpResourceToolCall},
	{"ListMcpResources", buildListMcpResourcesToolCall},
	{"StrReplace", buildStrReplaceToolCall},
	{"Shell", buildShellToolCall},
	{"Read", buildReadToolCall},
	{"Write", buildWriteToolCall},
	{"Delete", buildDeleteToolCall},
	{"Glob", buildGlobToolCall},
	{"Grep", buildGrepToolCall},
	{"ReadLints", buildReadLintsToolCall},
}

// parseToolArgs[T] unmarshals pc.Arguments into *T. Returns (args, errorMsg).
func parseToolArgs[T any](pc PendingToolCall) (*T, string) {
	var a T
	if err := json.Unmarshal([]byte(pc.Arguments), &a); err != nil {
		return nil, "parse " + pc.Name + " args: " + err.Error()
	}
	return &a, ""
}

// parseToolArgsPartial[T] unmarshals pc.Arguments into *T, ignoring errors.
// Use for tools where the schema may have extra fields.
func parseToolArgsPartial[T any](pc PendingToolCall) *T {
	var a T
	_ = json.Unmarshal([]byte(pc.Arguments), &a)
	return &a
}
