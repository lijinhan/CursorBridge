package agent

import (
	"testing"
)

func TestDispatchToolBuilder_Unknown(t *testing.T) {
	pc := PendingToolCall{ID: "1", Name: "UnknownTool", Arguments: "{}"}
	tool, errMsg := dispatchToolBuilder(nil, pc, toolBuilderRegistry)
	if tool != nil {
		t.Fatal("expected nil tool for unknown name")
	}
	if errMsg != "" {
		t.Fatalf("expected empty error for unknown name, got %q", errMsg)
	}
}

func TestDispatchToolBuilder_Shell(t *testing.T) {
	pc := PendingToolCall{
		ID:        "tc-1",
		Name:      "Shell",
		Arguments: `{"command":"echo hello","working_directory":"/tmp"}`,
	}
	tool, errMsg := dispatchToolBuilder(nil, pc, toolBuilderRegistry)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	shell := tool.GetShellToolCall()
	if shell == nil {
		t.Fatal("expected ShellToolCall")
	}
	if shell.Args.Command != "echo hello" {
		t.Fatalf("expected 'echo hello', got %q", shell.Args.Command)
	}
}

func TestDispatchToolBuilder_Read(t *testing.T) {
	pc := PendingToolCall{
		ID:        "tc-2",
		Name:      "Read",
		Arguments: `{"path":"/etc/hosts"}`,
	}
	tool, errMsg := dispatchToolBuilder(nil, pc, toolBuilderRegistry)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	read := tool.GetReadToolCall()
	if read == nil {
		t.Fatal("expected ReadToolCall")
	}
	if read.Args.Path != "/etc/hosts" {
		t.Fatalf("expected '/etc/hosts', got %q", read.Args.Path)
	}
}

func TestDispatchToolBuilder_Write(t *testing.T) {
	pc := PendingToolCall{
		ID:        "tc-3",
		Name:      "Write",
		Arguments: `{"path":"/tmp/test.txt","contents":"hello"}`,
	}
	tool, errMsg := dispatchToolBuilder(nil, pc, toolBuilderRegistry)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	edit := tool.GetEditToolCall()
	if edit == nil {
		t.Fatal("expected EditToolCall (Write maps to Edit)")
	}
	if edit.Args.Path != "/tmp/test.txt" {
		t.Fatalf("expected '/tmp/test.txt', got %q", edit.Args.Path)
	}
	if edit.Args.StreamContent == nil || *edit.Args.StreamContent != "hello" {
		t.Fatalf("expected stream_content 'hello', got %v", edit.Args.StreamContent)
	}
}

func TestDispatchToolBuilder_Glob(t *testing.T) {
	pc := PendingToolCall{
		ID:        "tc-4",
		Name:      "Glob",
		Arguments: `{"glob_pattern":"**/*.go"}`,
	}
	tool, errMsg := dispatchToolBuilder(nil, pc, toolBuilderRegistry)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	glob := tool.GetGlobToolCall()
	if glob == nil {
		t.Fatal("expected GlobToolCall")
	}
}

func TestDispatchToolBuilder_Grep(t *testing.T) {
	pc := PendingToolCall{
		ID:        "tc-5",
		Name:      "Grep",
		Arguments: `{"pattern":"TODO"}`,
	}
	tool, errMsg := dispatchToolBuilder(nil, pc, toolBuilderRegistry)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	grep := tool.GetGrepToolCall()
	if grep == nil {
		t.Fatal("expected GrepToolCall")
	}
}

func TestDispatchToolBuilder_StrReplace(t *testing.T) {
	pc := PendingToolCall{
		ID:        "tc-6",
		Name:      "StrReplace",
		Arguments: `{"path":"/nonexistent/file.txt","old_string":"a","new_string":"b"}`,
	}
	_, errMsg := dispatchToolBuilder(nil, pc, toolBuilderRegistry)
	// Should fail because the file doesn't exist
	if errMsg == "" {
		t.Fatal("expected error for nonexistent file")
	}
}
