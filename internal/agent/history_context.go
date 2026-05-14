package agent

import (
	"encoding/json"
	"fmt"
	"cursorbridge/internal/logutil"
	"cursorbridge/internal/strutil"
	"strings"
	"time"

	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"
)

// buildWorkspaceContext composes the <user_info> block Cursor's working app
// always sends as the first user-role message. We pull whatever the IDE
// shipped via UserMessageAction.RequestContext.Env (workspace path, OS
// version, shell, terminals folder, time zone). Returns empty string when
// the IDE didn't include any usable env data — better to omit the block
// than to send placeholders.
func buildWorkspaceContext(sess *Session) string {
	if sess.Action == nil {
		return ""
	}
	uma := sess.Action.GetUserMessageAction()
	if uma == nil {
		return ""
	}
	rc := uma.GetRequestContext()
	if rc == nil {
		return ""
	}
	env := rc.GetEnv()
	if env == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("<user_info>\n")
	if v := env.GetOsVersion(); v != "" {
		b.WriteString("OS Version: ")
		b.WriteString(v)
		b.WriteString("\n\n")
	}
	if v := env.GetShell(); v != "" {
		b.WriteString("Shell: ")
		b.WriteString(v)
		b.WriteString("\n\n")
	}
	if paths := env.GetWorkspacePaths(); len(paths) > 0 {
		b.WriteString("Workspace Path: ")
		b.WriteString(paths[0])
		b.WriteString("\n\n")
	}
	if v := env.GetProjectFolder(); v != "" {
		b.WriteString("Project Folder: ")
		b.WriteString(v)
		b.WriteString("\n\n")
	}
	tz := env.GetTimeZone()
	if tz == "" {
		tz = time.Local.String()
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}
	b.WriteString("Today's date: ")
	b.WriteString(time.Now().In(loc).Format("Monday Jan 2, 2006"))
	b.WriteString("\n\n")
	if v := env.GetTerminalsFolder(); v != "" {
		b.WriteString("Terminals folder: ")
		b.WriteString(v)
		b.WriteString("\n")
	}
	b.WriteString("</user_info>\n")

	// Append carry-forward context from ConversationState.RootPromptMessagesJson.
	if sess.State != nil {
		for _, blob := range sess.State.GetRootPromptMessagesJson() {
			if len(blob) == 0 {
				continue
			}
			var msg struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(blob, &msg); err == nil && msg.Content != "" {
				if !strings.Contains(msg.Content, "<user_info>") {
					b.WriteString("\n")
					b.WriteString(msg.Content)
					b.WriteString("\n")
				}
			}
		}
	}

	// Workspace context enrichment: SelectedContext (open files, highlighted
	// code, terminals, cursor rules) + RequestContext.ProjectLayouts (file
	// tree) are what the real Cursor IDE ships on every turn. Without these
	// the model can't answer "what file am I looking at" or "fix the thing
	// I just highlighted".
	appendSelectedContext(&b, sess)
	appendProjectTree(&b, sess)
	appendCursorRules(&b, sess)

	// Active plan — conversation-scoped PlanState survives across turns (a
	// fresh session built by BidiAppend on a new turn would otherwise have
	// empty Todos and trigger "no active plan — call CreatePlan first" on
	// the very next UpdateTodo). Echo the current state back on every turn
	// so the model keeps tracking progress instead of re-planning from
	// scratch each message.
	plan := PlanStateFor(sess.ConversationID)
	if plan == nil && (sess.PlanName != "" || len(sess.Todos) > 0) {
		// Session carries in-flight state the store might not have seen yet
		// (same-turn fallback when ConversationID is empty during recovery).
		plan = &PlanState{Name: sess.PlanName, Overview: sess.PlanOverview, Todos: sess.Todos}
	}
	if plan != nil && (plan.Name != "" || len(plan.Todos) > 0) {
		b.WriteString("\n<active_plan>\n")
		if plan.Name != "" {
			fmt.Fprintf(&b, "Name: %s\n", plan.Name)
		}
		if plan.Overview != "" {
			fmt.Fprintf(&b, "Overview: %s\n", plan.Overview)
		}
		if len(plan.Todos) > 0 {
			b.WriteString("TODOs:\n")
			for _, t := range plan.Todos {
				fmt.Fprintf(&b, "  - [%s] %s: %s\n", t.Status, t.ID, t.Content)
			}
		}
		b.WriteString("</active_plan>\n")
	}

	// Mirror the exact synthesised function names openAIToolsForRequest
	// produces so the model knows which real server/tool each "mcp_N__slug"
	// OpenAI function targets. DO NOT use CallMcpTool for these — the model
	// must call the synthesised function directly.
	if sess.McpMap != nil && len(sess.McpMap) > 0 {
		b.WriteString("\n<mcp_tools>\n")
		b.WriteString("Each MCP tool below is exposed as a separate OpenAI function. Call it by the exact `function` name shown — do NOT invent composite names like \"server-tool\", do NOT call CallMcpTool for these.\n\n")
		b.WriteString("These servers ARE connected. Do not call ListMcpResources or CallMcpTool to check — just invoke the functions below directly when you need them.\n\n")
		// Sort by idx for stable output.
		type row struct {
			fn         string
			serverName string
			serverID   string
			tool       string
		}
		rows := make([]row, 0, len(sess.McpMap))
		for fn, ref := range sess.McpMap {
			rows = append(rows, row{fn: fn, serverName: ref.ServerName, serverID: ref.ServerID, tool: ref.ToolName})
		}
		// Simple insertion sort on fn — slice tiny.
		for i := 1; i < len(rows); i++ {
			for j := i; j > 0 && rows[j-1].fn > rows[j].fn; j-- {
				rows[j-1], rows[j] = rows[j], rows[j-1]
			}
		}
		for _, r := range rows {
			fmt.Fprintf(&b, "- function=%s server=%q (id=%s) tool=%q\n", r.fn, r.serverName, r.serverID, r.tool)
		}
		b.WriteString("</mcp_tools>\n")
	}

	return b.String()
}

// ---- Workspace enrichment (SelectedContext + RequestContext) ----

// appendSelectedContext pulls in the UI-side picks the user made (attached
// files, highlighted code, captured terminals) so the model answers against
// what's actually on screen instead of hallucinating paths.
func appendSelectedContext(b *strings.Builder, sess *Session) {
	sc := selectedContextFor(sess)
	if sc == nil {
		return
	}

	// Editor state — Cursor packs the currently visible/focused files and the
	// cursor position into InvocationContext.IdeState. This is separate from
	// `Files` (which is explicit attachments); without reading both, the model
	// can't see "the file I'm currently looking at" unless the user manually
	// attaches it.
	//
	// We skip *.plan.md entries here: Cursor's Plan mode spawns a new one
	// for every CreatePlan call and auto-opens it. Feeding those back as
	// visible files doubled the prompt size and caused SSE timeouts; the
	// model already has the authoritative plan in <active_plan>.
	if ic := sc.GetInvocationContext(); ic != nil {
		if ide := ic.GetIdeState(); ide != nil {
			visible := ide.GetVisibleFiles()
			var kept []string
			for _, f := range visible {
				path := f.GetRelativePath()
				if path == "" {
					path = f.GetPath()
				}
				if isPlanArtifact(path) {
					continue
				}
				var entry strings.Builder
				fmt.Fprintf(&entry, "<visible_file path=%q total_lines=%d", path, f.GetTotalLines())
				if cp := f.GetCursorPosition(); cp != nil {
					fmt.Fprintf(&entry, " cursor_line=%d", cp.GetLine())
				}
				if cmd := f.GetActiveCommand(); cmd != "" {
					fmt.Fprintf(&entry, " active_command=%q", cmd)
				}
				entry.WriteString("/>")
				kept = append(kept, entry.String())
			}
			if len(kept) > 0 {
				b.WriteString("\n<editor_state>\n")
				for _, line := range kept {
					b.WriteString(line)
					b.WriteString("\n")
				}
				b.WriteString("</editor_state>\n")
			}
		}
	}

	// Open / attached files — send the whole file content for each one.
	files := sc.GetFiles()
	if len(files) > 0 {
		var entries []string
		for _, f := range files {
			path := f.GetRelativePath()
			if path == "" {
				path = f.GetPath()
			}
			if isPlanArtifact(path) {
				continue
			}
			content := strutil.Truncate(f.GetContent(), 16000)
			entries = append(entries, fmt.Sprintf("<file path=%q>\n%s\n</file>", path, content))
		}
		if len(entries) > 0 {
			b.WriteString("\n<open_files>\n")
			for _, e := range entries {
				b.WriteString(e)
				b.WriteString("\n")
			}
			b.WriteString("</open_files>\n")
		}
	}

	// Highlighted selections — include the line range so the model can cite it.
	sel := sc.GetCodeSelections()
	if len(sel) > 0 {
		b.WriteString("\n<code_selections>\n")
		for _, s := range sel {
			path := s.GetRelativePath()
			if path == "" {
				path = s.GetPath()
			}
			startLine, endLine := uint32(0), uint32(0)
			if r := s.GetRange(); r != nil {
				if p := r.GetStart(); p != nil {
					startLine = p.GetLine()
				}
				if p := r.GetEnd(); p != nil {
					endLine = p.GetLine()
				}
			}
			fmt.Fprintf(b, "<selection path=%q start=%d end=%d>\n%s\n</selection>\n",
				path, startLine, endLine, strutil.Truncate(s.GetContent(), 8000))
		}
		b.WriteString("</code_selections>\n")
	}

	// Attached terminals — whole terminal content (scrollback).
	terms := sc.GetTerminals()
	if len(terms) > 0 {
		b.WriteString("\n<terminals>\n")
		for _, t := range terms {
			title := t.GetTitle()
			if title == "" {
				title = "terminal"
			}
			fmt.Fprintf(b, "<terminal title=%q cwd=%q>\n%s\n</terminal>\n",
				title, t.GetPath(), strutil.Truncate(t.GetContent(), 8000))
		}
		b.WriteString("</terminals>\n")
	}

	// Terminal selections — user highlighted only a snippet of output.
	termSel := sc.GetTerminalSelections()
	if len(termSel) > 0 {
		b.WriteString("\n<terminal_selections>\n")
		for _, t := range termSel {
			title := t.GetTitle()
			if title == "" {
				title = "terminal"
			}
			fmt.Fprintf(b, "<terminal_selection title=%q>\n%s\n</terminal_selection>\n",
				title, strutil.Truncate(t.GetContent(), 4000))
		}
		b.WriteString("</terminal_selections>\n")
	}
}

// appendProjectTree emits a single-level directory overview from
// RequestContext.ProjectLayouts[0]. We keep it shallow (top-level dirs +
// file extension counts) because a recursive dump would blow the context
// budget on any real repo.
func appendProjectTree(b *strings.Builder, sess *Session) {
	rc := requestContextFor(sess)
	if rc == nil {
		return
	}
	layouts := rc.GetProjectLayouts()
	if len(layouts) == 0 {
		return
	}
	root := layouts[0]
	if root == nil {
		return
	}
	b.WriteString("\n<project_tree>\n")
	fmt.Fprintf(b, "root: %s\n", root.GetAbsPath())
	if root.GetNumFiles() > 0 {
		fmt.Fprintf(b, "total_files: %d\n", root.GetNumFiles())
	}
	if counts := root.GetFullSubtreeExtensionCounts(); len(counts) > 0 {
		b.WriteString("extensions:\n")
		for ext, n := range counts {
			fmt.Fprintf(b, "  %s: %d\n", ext, n)
		}
	}
	if dirs := root.GetChildrenDirs(); len(dirs) > 0 {
		b.WriteString("top_level_dirs:\n")
		for i, d := range dirs {
			if i >= 40 {
				fmt.Fprintf(b, "  … %d more\n", len(dirs)-i)
				break
			}
			fmt.Fprintf(b, "  - %s\n", filepathBase(d.GetAbsPath()))
		}
	}
	if files := root.GetChildrenFiles(); len(files) > 0 {
		b.WriteString("top_level_files:\n")
		for i, f := range files {
			if i >= 40 {
				fmt.Fprintf(b, "  … %d more\n", len(files)-i)
				break
			}
			fmt.Fprintf(b, "  - %s\n", f.GetName())
		}
	}
	b.WriteString("</project_tree>\n")
}

// appendCursorRules surfaces workspace-scoped AI rules (.cursor/rules,
// .cursorrules) and any attached/selected rule files. These are the
// user's hard directives for this project.
func appendCursorRules(b *strings.Builder, sess *Session) {
	rc := requestContextFor(sess)
	if rc == nil {
		logutil.Debug("appendCursorRules: requestContext is nil")
		return
	}
	rules := rc.GetRules()
	logutil.Debug("appendCursorRules: rules count", "count", len(rules))
	if len(rules) == 0 {
		return
	}
	b.WriteString("\n<cursor_rules>\n")
	b.WriteString("Workspace rules the user expects you to follow:\n")
	for _, r := range rules {
		name := filepathBase(r.GetFullPath())
		body := strutil.Truncate(r.GetContent(), 4000)
		if name != "" {
			fmt.Fprintf(b, "\n## %s\n", name)
		}
		if body != "" {
			b.WriteString(body)
			b.WriteString("\n")
		}
	}
	b.WriteString("</cursor_rules>\n")
}

func selectedContextFor(sess *Session) *agentv1.SelectedContext {
	if sess.Action == nil {
		return nil
	}
	uma := sess.Action.GetUserMessageAction()
	if uma == nil {
		return nil
	}
	msg := uma.GetUserMessage()
	if msg == nil {
		return nil
	}
	return msg.GetSelectedContext()
}

func requestContextFor(sess *Session) *agentv1.RequestContext {
	if sess.Action == nil {
		return nil
	}
	uma := sess.Action.GetUserMessageAction()
	if uma == nil {
		return nil
	}
	return uma.GetRequestContext()
}

// isPlanArtifact filters the .plan.md / Plans/* files Cursor auto-opens
// whenever Plan mode fires. Feeding them back as context doubles the
// prompt for zero value — the model already has the authoritative plan
// state in the <active_plan> block.
func isPlanArtifact(path string) bool {
	if path == "" {
		return false
	}
	lower := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	if strings.HasSuffix(lower, ".plan.md") {
		return true
	}
	if strings.Contains(lower, "/plans/") {
		return true
	}
	return false
}

func filepathBase(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}
