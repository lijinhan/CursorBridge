package agent

import (
	"testing"
)

func TestProviderFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://api.openai.com/v1", "openai"},
		{"https://api.anthropic.com/v1", "anthropic"},
		{"https://openrouter.ai/api/v1", "openrouter"},
		{"https://api.groq.com/v1", "groq"},
		{"https://api.together.xyz/v1", "together"},
		{"https://custom.example.com/v1", "custom"},
		{"", "custom"},
	}
	for _, tt := range tests {
		got := providerFromURL(tt.url)
		if got != tt.want {
			t.Errorf("providerFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
	}
	for _, tt := range tests {
		got := itoa(tt.input)
		if got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
