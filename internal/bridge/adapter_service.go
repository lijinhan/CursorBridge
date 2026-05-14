package bridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cursorbridge/internal/strutil"
)

func (s *ProxyService) TestAdapter(index int) (ModelAdapterConfig, error) {
	cfg, err := readConfig(s.cfgDir)
	if err != nil {
		return ModelAdapterConfig{}, err
	}
	if index < 0 || index >= len(cfg.ModelAdapters) {
		return ModelAdapterConfig{}, errors.New("adapter index out of range")
	}
	a := cfg.ModelAdapters[index]

	if a.APIKey == "" {
		a.LastTestResult = "missing API key"
	} else if a.BaseURL == "" {
		a.LastTestResult = "missing base URL"
	} else {
		a.LastTestResult = runAdapterPing(a)
	}

	a.LastTestedAt = time.Now().Unix()
	cfg.ModelAdapters[index] = a
	_ = s.SaveUserConfig(cfg)
	return a, nil
}

func runAdapterPing(a ModelAdapterConfig) string {
	if a.ModelID == "" {
		return "missing model ID"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	msg, err := runAdapterInferenceTest(ctx, a)
	if err != nil {
		return err.Error()
	}
	if strings.TrimSpace(msg) == "" {
		return "ok"
	}
	return "ok"
}

func runAdapterInferenceTest(ctx context.Context, a ModelAdapterConfig) (string, error) {
	base := strings.TrimRight(a.BaseURL, "/")
	var url string
	req := (*http.Request)(nil)
	var err error
	switch strings.ToLower(a.Type) {
	case "anthropic":
		url = base + "/v1/messages"
		payload := strings.NewReader(`{"model":` + strconv.Quote(a.ModelID) + `,"max_tokens":16,"messages":[{"role":"user","content":"Reply with OK only."}]}`)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, payload)
		if err != nil {
			return "", errors.New("invalid URL: " + err.Error())
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		url = base + "/chat/completions"
		payload := strings.NewReader(`{"model":` + strconv.Quote(a.ModelID) + `,"messages":[{"role":"user","content":"Reply with OK only."}],"max_tokens":16}`)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, payload)
		if err != nil {
			return "", errors.New("invalid URL: " + err.Error())
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+a.APIKey)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("network error: " + strutil.TruncateErr(err, 160))
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return strings.TrimSpace(string(body)), nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 140 {
		snippet = snippet[:140] + "…"
	}
	return "", fmt.Errorf("http %d — %s", resp.StatusCode, snippet)
}

