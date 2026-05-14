package agent

import (
	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"
)

func buildShellToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	type shellArgs struct {
		Command          string `json:"command"`
		WorkingDirectory string `json:"working_directory"`
		BlockUntilMs     int32  `json:"block_until_ms"`
		Description      string `json:"description"`
	}
	a := parseToolArgsPartial[shellArgs](pc)
	timeoutMs := a.BlockUntilMs
	if timeoutMs == 0 {
		timeoutMs = 5000
	}
	fileOutputThreshold := uint64(40000)
	hardTimeout := int32(86400000)
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_ShellToolCall{
			ShellToolCall: &agentv1.ShellToolCall{
				Args: &agentv1.ShellArgs{
					Command:          a.Command,
					WorkingDirectory: a.WorkingDirectory,
					Timeout:          timeoutMs,
					ToolCallId:       pc.ID,
					SimpleCommands:   []string{a.Command},
					ParsingResult: &agentv1.ShellCommandParsingResult{
						ExecutableCommands: []*agentv1.ShellCommandParsingResult_ExecutableCommand{
							{Name: a.Command, FullText: a.Command},
						},
					},
					FileOutputThresholdBytes: &fileOutputThreshold,
					SkipApproval:             true,
					TimeoutBehavior:           agentv1.TimeoutBehavior_TIMEOUT_BEHAVIOR_BACKGROUND,
					HardTimeout:              &hardTimeout,
				},
			},
		},
	}, ""
}

func buildGlobToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	type globArgs struct {
		GlobPattern     string  `json:"glob_pattern"`
		TargetDirectory *string `json:"target_directory,omitempty"`
	}
	a := parseToolArgsPartial[globArgs](pc)
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_GlobToolCall{
			GlobToolCall: &agentv1.GlobToolCall{
				Args: &agentv1.GlobToolArgs{
					GlobPattern:     a.GlobPattern,
					TargetDirectory: a.TargetDirectory,
				},
			},
		},
	}, ""
}

func buildGrepToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	type grepArgs struct {
		Pattern         string  `json:"pattern"`
		Path            *string `json:"path,omitempty"`
		Glob            *string `json:"glob,omitempty"`
		OutputMode      *string `json:"output_mode,omitempty"`
		ContextBefore   *int32  `json:"-B,omitempty"`
		ContextAfter    *int32  `json:"-A,omitempty"`
		Context         *int32  `json:"-C,omitempty"`
		CaseInsensitive *bool   `json:"-i,omitempty"`
		Type            *string `json:"type,omitempty"`
		HeadLimit       *int32  `json:"head_limit,omitempty"`
		Multiline       *bool   `json:"multiline,omitempty"`
	}
	a := parseToolArgsPartial[grepArgs](pc)
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_GrepToolCall{
			GrepToolCall: &agentv1.GrepToolCall{
				Args: &agentv1.GrepArgs{
					Pattern:         a.Pattern,
					Path:            a.Path,
					Glob:            a.Glob,
					OutputMode:      a.OutputMode,
					ContextBefore:   a.ContextBefore,
					ContextAfter:    a.ContextAfter,
					Context:         a.Context,
					CaseInsensitive: a.CaseInsensitive,
					Type:            a.Type,
					HeadLimit:       a.HeadLimit,
					Multiline:       a.Multiline,
				},
			},
		},
	}, ""
}

func buildReadLintsToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	type readLintsArgs struct {
		Paths []string `json:"paths"`
	}
	a := parseToolArgsPartial[readLintsArgs](pc)
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_ReadLintsToolCall{
			ReadLintsToolCall: &agentv1.ReadLintsToolCall{
				Args: &agentv1.ReadLintsToolArgs{
					Paths: a.Paths,
				},
			},
		},
	}, ""
}
