package agent

import (
	"os"
	"strings"

	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"
)

func buildStrReplaceToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	type strReplaceArgs struct {
		Path       string `json:"path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	a, errStr := parseToolArgs[strReplaceArgs](pc)
	if errStr != "" {
		return nil, errStr
	}
	if a.Path == "" {
		return nil, "StrReplace: path is required"
	}
	raw, readErr := os.ReadFile(a.Path)
	if readErr != nil {
		return nil, "StrReplace read file: " + readErr.Error()
	}
	before := string(raw)
	after, applyErr := applyStrReplace(before, a.OldString, a.NewString, a.ReplaceAll)
	if applyErr != "" {
		return nil, applyErr
	}
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_EditToolCall{
			EditToolCall: &agentv1.EditToolCall{
				Args: &agentv1.EditArgs{
					Path:          a.Path,
					StreamContent: &after,
				},
			},
		},
	}, ""
}

// applyStrReplace performs the string replacement logic shared by StrReplace
// and Write. Returns (content, errorMsg).
func applyStrReplace(before, oldStr, newStr string, replaceAll bool) (string, string) {
	// Try literal match first.
	if strings.Contains(before, oldStr) {
		if replaceAll {
			return strings.ReplaceAll(before, oldStr, newStr), ""
		}
		if strings.Count(before, oldStr) > 1 {
			return "", "StrReplace: old_string matches multiple locations — use replace_all or provide more context"
		}
		return strings.Replace(before, oldStr, newStr, 1), ""
	}
	// Normalise line endings and retry.
	normBefore := strings.ReplaceAll(before, "\r\n", "\n")
	normOld := strings.ReplaceAll(oldStr, "\r\n", "\n")
	if !strings.Contains(normBefore, normOld) {
		return "", "StrReplace: old_string not found"
	}
	if !replaceAll && strings.Count(normBefore, normOld) > 1 {
		return "", "StrReplace: old_string matches multiple locations — use replace_all or provide more context"
	}
	normNew := strings.ReplaceAll(newStr, "\r\n", "\n")
	if replaceAll {
		return strings.ReplaceAll(normBefore, normOld, normNew), ""
	}
	result := strings.Replace(normBefore, normOld, normNew, 1)
	// Preserve original line-ending style on write.
	if strings.Contains(before, "\r\n") {
		result = strings.ReplaceAll(result, "\n", "\r\n")
	}
	return result, ""
}

func buildReadToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	type readArgs struct {
		Path               string `json:"path"`
		Offset             *int32 `json:"offset,omitempty"`
		Limit              *int32 `json:"limit,omitempty"`
		IncludeLineNumbers *bool  `json:"include_line_numbers,omitempty"`
	}
	a := parseToolArgsPartial[readArgs](pc)
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_ReadToolCall{
			ReadToolCall: &agentv1.ReadToolCall{
				Args: &agentv1.ReadToolArgs{
					Path:               a.Path,
					Offset:             a.Offset,
					Limit:              a.Limit,
					IncludeLineNumbers: a.IncludeLineNumbers,
				},
			},
		},
	}, ""
}

func buildWriteToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	// Write maps to EditToolCall with stream_content set.
	// Schema uses `contents` (plural); accept `content` too.
	type writeArgs struct {
		Path     string `json:"path"`
		Contents string `json:"contents"`
		Content  string `json:"content"`
	}
	a := parseToolArgsPartial[writeArgs](pc)
	body := a.Contents
	if body == "" {
		body = a.Content
	}
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_EditToolCall{
			EditToolCall: &agentv1.EditToolCall{
				Args: &agentv1.EditArgs{
					Path:          a.Path,
					StreamContent: &body,
				},
			},
		},
	}, ""
}

func buildDeleteToolCall(pc PendingToolCall) (tool *agentv1.ToolCall, errMsg string) {
	type deleteArgs struct {
		Path string `json:"path"`
	}
	a := parseToolArgsPartial[deleteArgs](pc)
	return &agentv1.ToolCall{
		Tool: &agentv1.ToolCall_DeleteToolCall{
			DeleteToolCall: &agentv1.DeleteToolCall{
				Args: &agentv1.DeleteArgs{
					Path:       a.Path,
					ToolCallId: pc.ID,
				},
			},
		},
	}, ""
}
