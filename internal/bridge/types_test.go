package bridge

import (
	"encoding/json"
	"testing"
)

func TestModelAdapterConfigUnmarshalJSON_Number(t *testing.T) {
	raw := `{
		"displayName": "test",
		"type": "openai",
		"baseURL": "https://api.openai.com",
		"apiKey": "sk-xxx",
		"modelID": "gpt-4",
		"maxOutputTokens": 65536,
		"thinkingBudget": 16000,
		"retryCount": 3,
		"retryInterval": 2000,
		"timeout": 300000
	}`
	var cfg ModelAdapterConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal number format: %v", err)
	}
	if cfg.MaxOutputTokens != 65536 {
		t.Errorf("MaxOutputTokens = %d, want 65536", cfg.MaxOutputTokens)
	}
	if cfg.ThinkingBudget != 16000 {
		t.Errorf("ThinkingBudget = %d, want 16000", cfg.ThinkingBudget)
	}
	if cfg.RetryCount != 3 {
		t.Errorf("RetryCount = %d, want 3", cfg.RetryCount)
	}
	if cfg.RetryInterval != 2000 {
		t.Errorf("RetryInterval = %d, want 2000", cfg.RetryInterval)
	}
	if cfg.Timeout != 300000 {
		t.Errorf("Timeout = %d, want 300000", cfg.Timeout)
	}
}

func TestModelAdapterConfigUnmarshalJSON_String(t *testing.T) {
	raw := `{
		"displayName": "test",
		"type": "openai",
		"baseURL": "https://api.openai.com",
		"apiKey": "sk-xxx",
		"modelID": "gpt-4",
		"maxOutputTokens": "65536",
		"thinkingBudget": "16000",
		"retryCount": "3",
		"retryInterval": "2000",
		"timeout": "300000"
	}`
	var cfg ModelAdapterConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal string format: %v", err)
	}
	if cfg.MaxOutputTokens != 65536 {
		t.Errorf("MaxOutputTokens = %d, want 65536", cfg.MaxOutputTokens)
	}
	if cfg.RetryCount != 3 {
		t.Errorf("RetryCount = %d, want 3", cfg.RetryCount)
	}
}

func TestModelAdapterConfigUnmarshalJSON_EmptyString(t *testing.T) {
	raw := `{
		"displayName": "test",
		"type": "openai",
		"baseURL": "https://api.openai.com",
		"apiKey": "sk-xxx",
		"modelID": "gpt-4",
		"maxOutputTokens": "",
		"thinkingBudget": "",
		"retryCount": "",
		"retryInterval": "",
		"timeout": ""
	}`
	var cfg ModelAdapterConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal empty string format: %v", err)
	}
	if cfg.MaxOutputTokens != 0 {
		t.Errorf("MaxOutputTokens = %d, want 0", cfg.MaxOutputTokens)
	}
	if cfg.RetryCount != 0 {
		t.Errorf("RetryCount = %d, want 0", cfg.RetryCount)
	}
}

func TestModelAdapterConfigUnmarshalJSON_Missing(t *testing.T) {
	raw := `{
		"displayName": "test",
		"type": "openai",
		"baseURL": "https://api.openai.com",
		"apiKey": "sk-xxx",
		"modelID": "gpt-4"
	}`
	var cfg ModelAdapterConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal missing fields: %v", err)
	}
	if cfg.MaxOutputTokens != 0 {
		t.Errorf("MaxOutputTokens = %d, want 0", cfg.MaxOutputTokens)
	}
	if cfg.RetryCount != 0 {
		t.Errorf("RetryCount = %d, want 0", cfg.RetryCount)
	}
}
