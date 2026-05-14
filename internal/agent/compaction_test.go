package agent

import "testing"

func TestGetModelContextTokenLimit_ExplicitConfig(t *testing.T) {
	opts := AdapterOpts{ContextTokenLimit: 50000}
	got := getModelContextTokenLimit(opts, "gpt-4o")
	if got != 50000 {
		t.Errorf("expected 50000, got %d", got)
	}
}

func TestGetModelContextTokenLimit_KnownModel(t *testing.T) {
	opts := AdapterOpts{}
	got := getModelContextTokenLimit(opts, "gpt-4o")
	if got != 128000 {
		t.Errorf("expected 128000 for gpt-4o, got %d", got)
	}
}

func TestGetModelContextTokenLimit_KnownModelCaseInsensitive(t *testing.T) {
	opts := AdapterOpts{}
	got := getModelContextTokenLimit(opts, "GPT-4O-2024-08-06")
	if got != 128000 {
		t.Errorf("expected 128000 for GPT-4O-2024-08-06, got %d", got)
	}
}

func TestGetModelContextTokenLimit_AnthropicModel(t *testing.T) {
	opts := AdapterOpts{}
	got := getModelContextTokenLimit(opts, "claude-sonnet-4-5-20250514")
	if got != 200000 {
		t.Errorf("expected 200000 for claude-sonnet-4-5, got %d", got)
	}
}

func TestGetModelContextTokenLimit_UnknownModel(t *testing.T) {
	opts := AdapterOpts{}
	got := getModelContextTokenLimit(opts, "some-unknown-model")
	if got != defaultContextTokenLimit {
		t.Errorf("expected default %d for unknown model, got %d", defaultContextTokenLimit, got)
	}
}

func TestGetModelContextTokenLimit_EmptyModel(t *testing.T) {
	opts := AdapterOpts{}
	got := getModelContextTokenLimit(opts, "")
	if got != defaultContextTokenLimit {
		t.Errorf("expected default %d for empty model, got %d", defaultContextTokenLimit, got)
	}
}

func TestGetModelContextTokenLimit_ConfigOverridesKnown(t *testing.T) {
	opts := AdapterOpts{ContextTokenLimit: 64000}
	got := getModelContextTokenLimit(opts, "gpt-4o")
	if got != 64000 {
		t.Errorf("expected explicit 64000 to override known default, got %d", got)
	}
}

func TestLookupModelTokenLimit(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"gpt-4o", 128000},
		{"gpt-4-turbo-2024-04-09", 128000},
		{"gpt-4-0314", 8192},
		{"gpt-4-32k", 32768},
		{"gpt-3.5-turbo", 16385},
		{"o1-preview", 200000},
		{"o3-mini", 200000},
		{"o4-mini", 200000},
		{"claude-sonnet-4-5-20250514", 200000},
		{"claude-opus-4-20250514", 200000},
		{"claude-3.5-sonnet-20241022", 200000},
		{"claude-3-opus-20240229", 200000},
		{"gemini-2.5-pro-preview-05-06", 1048576},
		{"deepseek-chat", 131072},
		{"deepseek-reasoner", 131072},
		{"mistral-large-latest", 131072},
		{"unknown-model", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := lookupModelTokenLimit(tt.model)
		if got != tt.want {
			t.Errorf("lookupModelTokenLimit(%q) = %d, want %d", tt.model, got, tt.want)
		}
	}
}

func TestIsContextOverflowError(t *testing.T) {
	tests := []struct {
		errMsg string
		want   bool
	}{
		{"context_length_exceeded", true},
		{"This model's maximum context length is 4096 tokens", true},
		{"too many tokens in the request", true},
		{"context window exceeded", true},
		{"token limit reached", true},
		{"input is too long", true},
		{"reduce the length of the messages", true},
		{"exceeds the maximum number of tokens", true},
		{"request too large", true},
		{"rate limit exceeded", false},
		{"internal server error", false},
		{"", false},
	}
	for _, tt := range tests {
		var err error
		if tt.errMsg != "" {
			err = &testError{msg: tt.errMsg}
		}
		got := isContextOverflowError(err)
		if got != tt.want {
			t.Errorf("isContextOverflowError(%q) = %v, want %v", tt.errMsg, got, tt.want)
		}
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestMergeSummaryWithExisting(t *testing.T) {
	got := mergeSummaryWithExisting("", "new summary")
	if got != "new summary" {
		t.Errorf("expected 'new summary', got %q", got)
	}

	got = mergeSummaryWithExisting("existing", "new summary")
	if got != "existing\n\n[Earlier context summary]:\nnew summary" {
		t.Errorf("unexpected merged result: %q", got)
	}
}

func TestBuildCompactionArchive(t *testing.T) {
	turns := []*ConvTurn{
		{User: "hello", Assistant: "hi"},
		{User: "how are you", Assistant: "fine"},
	}
	archive := buildCompactionArchive("summary text", turns, 1)
	if archive.TurnCount != 1 {
		t.Errorf("expected TurnCount=1, got %d", archive.TurnCount)
	}
	if archive.WindowTail != 1 {
		t.Errorf("expected WindowTail=1, got %d", archive.WindowTail)
	}
	if archive.Summary != "summary text" {
		t.Errorf("expected Summary='summary text', got %q", archive.Summary)
	}
	if archive.CharSaved != -5 {
		t.Errorf("expected CharSaved=-5, got %d", archive.CharSaved)
	}
}
