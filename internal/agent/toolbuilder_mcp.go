package agent

import (
	"encoding/json"
	"strings"

	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"

	"google.golang.org/protobuf/types/known/structpb"
)

// buildMCPToolCall re-routes synthesized per-MCP-tool functions
// (mcp_<idx>__<slug>) into a proper CallMcpTool with the real server identifier.
// The model sees each MCP tool as its own OpenAI function, which kills the "let
// me invent a composite tool name" failure mode.
func buildMCPToolCall(sess *Session, pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	if sess == nil || sess.McpMap == nil {
		return nil, "unknown MCP tool: " + pc.Name
	}
	ref := sess.McpMap[pc.Name]
	if ref == nil {
		return nil, "unknown MCP tool: " + pc.Name
	}
	var rawArgs map[string]interface{}
	if strings.TrimSpace(pc.Arguments) != "" {
		if err := json.Unmarshal([]byte(pc.Arguments), &rawArgs); err != nil {
			return nil, "parse MCP args: " + err.Error()
		}
	}
	qualifiedKey := ref.ToolName
	if ref.ServerID != "" && !strings.HasPrefix(ref.ToolName, ref.ServerID+"-") {
		qualifiedKey = ref.ServerID + "-" + ref.ToolName
	}
	mcpArgs := &agentv1.McpArgs{
		Name:               qualifiedKey,
		ToolName:           ref.ToolName,
		ToolCallId:         pc.ID,
		ProviderIdentifier: ref.ServerID,
	}
	if len(rawArgs) > 0 {
		mcpArgs.Args = make(map[string]*structpb.Value)
		for k, v := range rawArgs {
			if sv, err := structpb.NewValue(v); err == nil {
				mcpArgs.Args[k] = sv
			}
		}
	}
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_McpToolCall{
			McpToolCall: &agentv1.McpToolCall{
				Args: mcpArgs,
			},
		},
	}, ""
}

func buildCallMcpToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	type callMcpArgs struct {
		Server    string                 `json:"server"`
		ToolName  string                 `json:"toolName"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	a, err := parseToolArgs[callMcpArgs](pc)
	if err != "" {
		return nil, err
	}
	mcpArgs := &agentv1.McpArgs{
		Name:               a.Server,
		ToolName:           a.ToolName,
		ToolCallId:         pc.ID,
		ProviderIdentifier: a.Server,
	}
	if len(a.Arguments) > 0 {
		mcpArgs.Args = make(map[string]*structpb.Value)
		for k, v := range a.Arguments {
			sv, err := structpb.NewValue(v)
			if err == nil {
				mcpArgs.Args[k] = sv
			}
		}
	}
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_McpToolCall{
			McpToolCall: &agentv1.McpToolCall{
				Args: mcpArgs,
			},
		},
	}, ""
}

func buildFetchMcpResourceToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	type fetchMcpArgs struct {
		Server       string `json:"server"`
		URI          string `json:"uri"`
		DownloadPath string `json:"downloadPath"`
	}
	a, err := parseToolArgs[fetchMcpArgs](pc)
	if err != "" {
		return nil, err
	}
	readArgs := &agentv1.ReadMcpResourceExecArgs{
		Server: a.Server,
		Uri:    a.URI,
	}
	if a.DownloadPath != "" {
		readArgs.DownloadPath = &a.DownloadPath
	}
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_ReadMcpResourceToolCall{
			ReadMcpResourceToolCall: &agentv1.ReadMcpResourceToolCall{
				Args: readArgs,
			},
		},
	}, ""
}

func buildListMcpResourcesToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	type listMcpArgs struct {
		Server string `json:"server"`
	}
	a := parseToolArgsPartial[listMcpArgs](pc)
	args := &agentv1.ListMcpResourcesExecArgs{}
	if a.Server != "" {
		args.Server = &a.Server
	}
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_ListMcpResourcesToolCall{
			ListMcpResourcesToolCall: &agentv1.ListMcpResourcesToolCall{
				Args: args,
			},
		},
	}, ""
}
