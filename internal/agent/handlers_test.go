package agent

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestErrResult(t *testing.T) {
	r := errResult(http.StatusBadRequest, "bad input")
	if r.Status != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", r.Status, http.StatusBadRequest)
	}
	if r.ContentType != "application/json" {
		t.Errorf("contentType: got %q", r.ContentType)
	}
	if len(r.Body) == 0 {
		t.Error("empty body")
	}
}

func TestTextMessage(t *testing.T) {
	msg := textMessage("user", "hello")
	if msg.Role != "user" {
		t.Errorf("role: got %q", msg.Role)
	}
	var s string
	if err := json.Unmarshal(msg.Content, &s); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if s != "hello" {
		t.Errorf("content: got %q, want %q", s, "hello")
	}
}

func TestStripCodeFence(t *testing.T) {
	if got := stripCodeFence("hello"); got != "hello" {
		t.Errorf("no fence: got %q", got)
	}
	if got := stripCodeFence("```json\n{\"a\":1}\n```"); got != `{"a":1}` {
		t.Errorf("json fence: got %q", got)
	}
	if got := stripCodeFence("```\ncode\n```"); got != "code" {
		t.Errorf("plain fence: got %q", got)
	}
}

func TestExtractJSONArray(t *testing.T) {
	if got := extractJSONArray(`[1,2,3]`); got != `[1,2,3]` {
		t.Errorf("simple: got %q", got)
	}
	if got := extractJSONArray("prefix [1,2] suffix"); got != "[1,2]" {
		t.Errorf("embedded: got %q", got)
	}
	if got := extractJSONArray("```json\n[1,2]\n```"); got != "[1,2]" {
		t.Errorf("fenced: got %q", got)
	}
}

func TestExtractJSONObject(t *testing.T) {
	if got := extractJSONObject(`{"a":1}`); got != `{"a":1}` {
		t.Errorf("simple: got %q", got)
	}
	if got := extractJSONObject("prefix {\"a\":1} suffix"); got != `{"a":1}` {
		t.Errorf("embedded: got %q", got)
	}
}

func TestDefaultString(t *testing.T) {
	if got := defaultString("", "fallback"); got != "fallback" {
		t.Errorf("empty: got %q", got)
	}
	if got := defaultString("value", "fallback"); got != "value" {
		t.Errorf("non-empty: got %q", got)
	}
	if got := defaultString("  ", "fallback"); got != "fallback" {
		t.Errorf("whitespace: got %q", got)
	}
}

func TestMaxInt32(t *testing.T) {
	if got := maxInt32(5, 1); got != 5 {
		t.Errorf("5,1: got %d", got)
	}
	if got := maxInt32(0, 1); got != 1 {
		t.Errorf("0,1: got %d", got)
	}
}

func TestParseBugBotResponse_NoIssues(t *testing.T) {
	bugs, summary := parseBugBotResponse("NO_ISSUES")
	if bugs != nil {
		t.Errorf("expected nil bugs, got %v", bugs)
	}
	if summary != "No issues found" {
		t.Errorf("summary: got %q", summary)
	}
	bugs, summary = parseBugBotResponse("")
	if bugs != nil {
		t.Errorf("expected nil bugs for empty, got %v", bugs)
	}
}

func TestParseBugBotResponse_JSONArray(t *testing.T) {
	input := `[{"title":"Bug","description":"desc","severity":"high","confidence":0.9,"rationale":"bad","file":"a.go","start_line":1,"end_line":5}]`
	bugs, _ := parseBugBotResponse(input)
	if len(bugs) != 1 {
		t.Fatalf("expected 1 bug, got %d", len(bugs))
	}
	if bugs[0].GetTitle() != "Bug" {
		t.Errorf("title: got %q", bugs[0].GetTitle())
	}
}

func TestPickProviderStreamer(t *testing.T) {
	s := pickProviderStreamer("anthropic")
	if s == nil {
		t.Error("nil for anthropic")
	}
	s = pickProviderStreamer("openai")
	if s == nil {
		t.Error("nil for openai")
	}
	s = pickProviderStreamer("")
	if s == nil {
		t.Error("nil for empty")
	}
}
