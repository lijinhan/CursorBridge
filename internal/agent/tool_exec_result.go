package agent

import (
	"regexp"
	"strings"

	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"
)

// attachToolResultToProto writes the accumulated tool result onto the
// matching field of the ToolCall proto so the ToolCallCompleted frame
// carries it. Cursor's UI reads the embedded result to render the tool
// output inside the pill — without this the pill shows just the command
// header and leaves the result section blank. Best-effort: silently
// no-ops on unsupported combinations (model still sees the result via
// the tool-role message it gets fed afterwards).
func attachToolResultToProto(tc *agentv1.ToolCall, toolName string, env *toolResultEnvelope) {
	if tc == nil || env == nil {
		return
	}
	if strings.HasPrefix(toolName, "mcp_") {
		attachMcpResult(tc, env)
		return
	}
	switch toolName {
	case "Shell":
		attachShellResult(tc, env)
	case "Write":
		attachWriteResult(tc, env)
	case "Read":
		attachReadResult(tc, env)
	case "Glob":
		attachGlobResult(tc, env)
	case "Grep":
		attachGrepResult(tc, env)
	case "Delete":
		attachDeleteResult(tc, env)
	case "StrReplace":
		attachWriteResult(tc, env)
	case "CallMcpTool":
		attachMcpResult(tc, env)
	case "FetchMcpResource":
		attachFetchMcpResult(tc, env)
	case "ListMcpResources":
		attachListMcpResult(tc, env)
	case "ReadLints":
		attachReadLintsResult(tc, env)
	}
}

func attachShellResult(tc *agentv1.ToolCall, env *toolResultEnvelope) {
	shell := tc.GetShellToolCall()
	if shell == nil || env.ShellAccum == nil {
		return
	}
	cmd := ""
	if shell.Args != nil {
		cmd = shell.Args.GetCommand()
	}
	shell.Result = &agentv1.ShellResult{
		Result: &agentv1.ShellResult_Success{
			Success: &agentv1.ShellSuccess{
				Command:          cmd,
				WorkingDirectory: env.ShellAccum.Cwd,
				ExitCode:         int32(env.ShellAccum.ExitCode),
				Stdout:           string(env.ShellAccum.Stdout),
				Stderr:           string(env.ShellAccum.Stderr),
			},
		},
	}
}

func attachWriteResult(tc *agentv1.ToolCall, env *toolResultEnvelope) {
	edit := tc.GetEditToolCall()
	if edit == nil || env.ExecClient == nil {
		return
	}
	wr := env.ExecClient.GetWriteResult()
	if wr == nil {
		return
	}
	// Map WriteResult variants onto EditResult so the pill can render
	// success/permission-denied/etc consistently.
	er := &agentv1.EditResult{}
	switch v := wr.GetResult().(type) {
	case *agentv1.WriteResult_Success:
		path := ""
		afterContent := ""
		if edit.Args != nil {
			path = edit.Args.GetPath()
			if edit.Args.StreamContent != nil {
				afterContent = *edit.Args.StreamContent
			}
		}
		linesAdded := int32(0)
		for _, ch := range afterContent {
			if ch == '\n' {
				linesAdded++
			}
		}
		if afterContent != "" {
			linesAdded++
		}
		er.Result = &agentv1.EditResult_Success{
			Success: &agentv1.EditSuccess{
				Path:                 path,
				LinesAdded:           &linesAdded,
				AfterFullFileContent: afterContent,
			},
		}
		_ = v
	case *agentv1.WriteResult_PermissionDenied:
		er.Result = &agentv1.EditResult_WritePermissionDenied{
			WritePermissionDenied: &agentv1.EditWritePermissionDenied{
				Path: v.PermissionDenied.GetPath(),
			},
		}
	case *agentv1.WriteResult_Error:
		er.Result = &agentv1.EditResult_Error{
			Error: &agentv1.EditError{Error: v.Error.GetError()},
		}
	case *agentv1.WriteResult_Rejected:
		er.Result = &agentv1.EditResult_Rejected{
			Rejected: &agentv1.EditRejected{
				Path:   v.Rejected.GetPath(),
				Reason: v.Rejected.GetReason(),
			},
		}
	}
	edit.Result = er
}

func attachReadResult(tc *agentv1.ToolCall, env *toolResultEnvelope) {
	read := tc.GetReadToolCall()
	if read == nil || env.ExecClient == nil {
		return
	}
	rr := env.ExecClient.GetReadResult()
	if rr == nil {
		return
	}
	// ReadResult (from Cursor) uses ReadSuccess/ReadError; ReadToolCall
	// wants ReadToolSuccess/ReadToolError which are separate types with
	// partially overlapping fields. Map success via the path+content we
	// care about; errors via the single error_message field.
	rtr := &agentv1.ReadToolResult{}
	switch v := rr.GetResult().(type) {
	case *agentv1.ReadResult_Success:
		rtr.Result = &agentv1.ReadToolResult_Success{
			Success: &agentv1.ReadToolSuccess{
				Path:       v.Success.GetPath(),
				TotalLines: uint32(v.Success.GetTotalLines()),
				Output:     &agentv1.ReadToolSuccess_Content{Content: v.Success.GetContent()},
			},
		}
	case *agentv1.ReadResult_Error:
		rtr.Result = &agentv1.ReadToolResult_Error{
			Error: &agentv1.ReadToolError{ErrorMessage: v.Error.GetError()},
		}
	default:
		return
	}
	read.Result = rtr
}

func attachGlobResult(tc *agentv1.ToolCall, env *toolResultEnvelope) {
	glob := tc.GetGlobToolCall()
	if glob == nil || env.ExecClient == nil {
		return
	}
	ls := env.ExecClient.GetLsResult()
	if ls == nil {
		return
	}
	pattern := ""
	rootPath := "."
	if glob.Args != nil {
		pattern = glob.Args.GetGlobPattern()
		if glob.Args.TargetDirectory != nil {
			rootPath = *glob.Args.TargetDirectory
		}
	}
	switch v := ls.GetResult().(type) {
	case *agentv1.LsResult_Success:
		files := collectGlobMatches(v.Success.GetDirectoryTreeRoot(), rootPath, pattern)
		total := int32(len(files))
		glob.Result = &agentv1.GlobToolResult{
			Result: &agentv1.GlobToolResult_Success{
				Success: &agentv1.GlobToolSuccess{
					Pattern:    pattern,
					Path:       rootPath,
					Files:      files,
					TotalFiles: total,
				},
			},
		}
	case *agentv1.LsResult_Error:
		glob.Result = &agentv1.GlobToolResult{
			Result: &agentv1.GlobToolResult_Error{
				Error: &agentv1.GlobToolError{Error: v.Error.GetError()},
			},
		}
	}
}

// collectGlobMatches walks the Ls directory tree Cursor returned and yields
// every file whose relative path matches the glob pattern. Uses the
// minimal globToRegex matcher below (supports *, **, ? — the 99% case).
// An empty pattern returns every file.
func collectGlobMatches(root *agentv1.LsDirectoryTreeNode, rootPath, pattern string) []string {
	if root == nil {
		return nil
	}
	re := globToRegex(pattern)
	var out []string
	var walk func(n *agentv1.LsDirectoryTreeNode)
	walk = func(n *agentv1.LsDirectoryTreeNode) {
		if n == nil {
			return
		}
		base := n.GetAbsPath()
		for _, f := range n.GetChildrenFiles() {
			full := base
			if full != "" && !strings.HasSuffix(full, "/") && !strings.HasSuffix(full, "\\") {
				full += "/"
			}
			full += f.GetName()
			rel := full
			if rootPath != "" && rootPath != "." && strings.HasPrefix(rel, rootPath) {
				rel = strings.TrimPrefix(rel, rootPath)
				rel = strings.TrimPrefix(rel, "/")
				rel = strings.TrimPrefix(rel, "\\")
			}
			if re == nil || re.MatchString(rel) || re.MatchString(full) {
				out = append(out, full)
			}
		}
		for _, c := range n.GetChildrenDirs() {
			walk(c)
		}
	}
	walk(root)
	return out
}

// globToRegex compiles a minimal glob pattern into a regexp. Supports the
// three operators models overwhelmingly emit: `**` (any dirs), `*` (any
// chars except separators), and `?` (single char). Returns nil for empty
// patterns so the caller can short-circuit to "match all".
func globToRegex(pattern string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	var b strings.Builder
	b.WriteString("^")
	// Normalise backslashes to forward so the same regex works for both.
	p := strings.ReplaceAll(pattern, "\\", "/")
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch c {
		case '*':
			if i+1 < len(p) && p[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '/':
			b.WriteString("[/\\\\]")
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	return re
}

func attachGrepResult(tc *agentv1.ToolCall, env *toolResultEnvelope) {
	grep := tc.GetGrepToolCall()
	if grep == nil || env.ExecClient == nil {
		return
	}
	if r := env.ExecClient.GetGrepResult(); r != nil {
		grep.Result = r
	}
}

func attachMcpResult(tc *agentv1.ToolCall, env *toolResultEnvelope) {
	mcp := tc.GetMcpToolCall()
	if mcp == nil || env.ExecClient == nil {
		return
	}
	if r := env.ExecClient.GetMcpResult(); r != nil {
		mcp.Result = &agentv1.McpToolResult{}
		switch v := r.GetResult().(type) {
		case *agentv1.McpResult_Success:
			mcp.Result.Result = &agentv1.McpToolResult_Success{
				Success: v.Success,
			}
		case *agentv1.McpResult_Error:
			mcp.Result.Result = &agentv1.McpToolResult_Error{
				Error: &agentv1.McpToolError{Error: v.Error.GetError()},
			}
		}
	}
}

func attachFetchMcpResult(tc *agentv1.ToolCall, env *toolResultEnvelope) {
	fetch := tc.GetReadMcpResourceToolCall()
	if fetch == nil || env.ExecClient == nil {
		return
	}
	if r := env.ExecClient.GetReadMcpResourceExecResult(); r != nil {
		fetch.Result = r
	}
}

func attachListMcpResult(tc *agentv1.ToolCall, env *toolResultEnvelope) {
	list := tc.GetListMcpResourcesToolCall()
	if list == nil || env.ExecClient == nil {
		return
	}
	if r := env.ExecClient.GetListMcpResourcesExecResult(); r != nil {
		list.Result = r
	}
}

func attachDeleteResult(tc *agentv1.ToolCall, env *toolResultEnvelope) {
	del := tc.GetDeleteToolCall()
	if del == nil || env.ExecClient == nil {
		return
	}
	if r := env.ExecClient.GetDeleteResult(); r != nil {
		del.Result = r
	}
}

func attachReadLintsResult(tc *agentv1.ToolCall, env *toolResultEnvelope) {
	rl := tc.GetReadLintsToolCall()
	if rl == nil || env.ExecClient == nil {
		return
	}
	dr := env.ExecClient.GetDiagnosticsResult()
	if dr == nil {
		return
	}
	// Map DiagnosticsResult onto ReadLintsToolResult.
	// DiagnosticsSuccess uses []*Diagnostic (Range type *Range) while
	// ReadLintsToolSuccess uses []*DiagnosticItem (Range type
	// *DiagnosticRange). The fields overlap enough that we can build
	// DiagnosticItem entries from the Diagnostic data.
	rtr := &agentv1.ReadLintsToolResult{}
	switch v := dr.GetResult().(type) {
	case *agentv1.DiagnosticsResult_Success:
		s := v.Success
		items := make([]*agentv1.DiagnosticItem, 0, len(s.GetDiagnostics()))
		for _, d := range s.GetDiagnostics() {
			item := &agentv1.DiagnosticItem{
				Severity: d.GetSeverity(),
				Message:  d.GetMessage(),
				Source:   d.GetSource(),
				Code:     d.GetCode(),
				IsStale:  d.GetIsStale(),
			}
			if r := d.GetRange(); r != nil {
				item.Range = &agentv1.DiagnosticRange{
					Start: r.GetStart(),
					End:   r.GetEnd(),
				}
			}
			items = append(items, item)
		}
		fd := &agentv1.FileDiagnostics{
			Path:             s.GetPath(),
			Diagnostics:      items,
			DiagnosticsCount: int32(len(items)),
		}
		rtr.Result = &agentv1.ReadLintsToolResult_Success{
			Success: &agentv1.ReadLintsToolSuccess{
				FileDiagnostics:  []*agentv1.FileDiagnostics{fd},
				TotalFiles:       1,
				TotalDiagnostics: s.GetTotalDiagnostics(),
			},
		}
	case *agentv1.DiagnosticsResult_Error:
		rtr.Result = &agentv1.ReadLintsToolResult_Error{
			Error: &agentv1.ReadLintsToolError{ErrorMessage: v.Error.GetError()},
		}
	default:
		return
	}
	rl.Result = rtr
}
