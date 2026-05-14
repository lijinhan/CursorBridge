package agent

import (
	"testing"
)

func TestApplyStrReplace_LiteralMatch(t *testing.T) {
	before := "hello world"
	after, errMsg := applyStrReplace(before, "world", "Go", false)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if after != "hello Go" {
		t.Fatalf("expected 'hello Go', got %q", after)
	}
}

func TestApplyStrReplace_ReplaceAll(t *testing.T) {
	before := "aaa bbb aaa"
	after, errMsg := applyStrReplace(before, "aaa", "ccc", true)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if after != "ccc bbb ccc" {
		t.Fatalf("expected 'ccc bbb ccc', got %q", after)
	}
}

func TestApplyStrReplace_MultipleWithoutReplaceAll(t *testing.T) {
	before := "aaa bbb aaa"
	_, errMsg := applyStrReplace(before, "aaa", "ccc", false)
	if errMsg == "" {
		t.Fatal("expected error for multiple matches without replace_all")
	}
}

func TestApplyStrReplace_NotFound(t *testing.T) {
	before := "hello world"
	_, errMsg := applyStrReplace(before, "xyz", "abc", false)
	if errMsg == "" {
		t.Fatal("expected error for not found")
	}
}

func TestApplyStrReplace_LineEndingNormalization(t *testing.T) {
	before := "line1\r\nline2\r\nline3"
	after, errMsg := applyStrReplace(before, "line2", "LINE2", false)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	want := "line1\r\nLINE2\r\nline3"
	if after != want {
		t.Fatalf("expected %q, got %q", want, after)
	}
}

func TestApplyStrReplace_CRLFMismatch(t *testing.T) {
	before := "line1\r\nline2\r\nline3"
	after, errMsg := applyStrReplace(before, "line1\nline2", "REPLACED", false)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	// Should normalize \r\n → \n for matching, then preserve original endings
	if after != "REPLACED\r\nline3" {
		t.Fatalf("unexpected result: %q", after)
	}
}
