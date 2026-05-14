package agent

import (
	"context"
	"net/http"
	"strings"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func HandleGetTerminalCompletion(ctx context.Context, reqBody []byte, contentType string, resolve AdapterResolver, selectedModel string) Result {
	req := &aiserverv1.GetTerminalCompletionRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		return errResult(http.StatusBadRequest, "decode terminal completion: "+err.Error())
	}
	target, ok := ResolveAdapterForModel(resolve.Resolve(), selectedModel)
	if !ok {
		return errResult(http.StatusBadRequest, "no BYOK adapter configured")
	}
	stream := pickProviderStreamer(target.ProviderType)
	command, err := generateTerminalCompletion(ctx, req, target.BaseURL, target.APIKey, target.Model, target.Opts, stream)
	if err != nil {
		return errResult(http.StatusBadGateway, "generate terminal completion: "+err.Error())
	}
	body, err := proto.Marshal(&aiserverv1.GetTerminalCompletionResponse{Command: command})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal terminal completion response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}

func generateTerminalCompletion(ctx context.Context, req *aiserverv1.GetTerminalCompletionRequest, baseURL, apiKey, model string, opts AdapterOpts, stream providerStreamer) (string, error) {
	prompt := buildTerminalCompletionPrompt(req)
	fastOpts := opts
	fastOpts.ReasoningEffort = ""
	fastOpts.ServiceTier = ""
	if fastOpts.MaxOutputTokens <= 0 || fastOpts.MaxOutputTokens > 200 {
		fastOpts.MaxOutputTokens = 200
	}
	result, err := collectSingleResponse(ctx, stream, baseURL, apiKey, model, fastOpts, []openAIMessage{
		textMessage("system", "You are a terminal command completion assistant. Given a partial command and context, complete the command. Return ONLY the completed command text, nothing else. No explanation, no quotes."),
		textMessage("user", prompt),
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

func buildTerminalCompletionPrompt(req *aiserverv1.GetTerminalCompletionRequest) string {
	var b strings.Builder
	b.WriteString("Complete this terminal command:\n\n")
	b.WriteString("Partial command: ")
	b.WriteString(req.GetCurrentCommand())
	b.WriteString("\n")
	if platform := req.GetUserPlatform(); platform != "" {
		b.WriteString("Platform: ")
		b.WriteString(platform)
		b.WriteString("\n")
	}
	if folder := req.GetCurrentFolder(); folder != "" {
		b.WriteString("Current directory: ")
		b.WriteString(folder)
		b.WriteString("\n")
	}
	if len(req.GetCommandHistory()) > 0 {
		b.WriteString("Recent commands:\n")
		for i, cmd := range req.GetCommandHistory() {
			if i > 10 {
				break
			}
			b.WriteString("  ")
			b.WriteString(cmd)
			b.WriteString("\n")
		}
	}
	if len(req.GetPastResults()) > 0 {
		b.WriteString("Previous completions (avoid repeating):\n")
		for _, r := range req.GetPastResults() {
			b.WriteString("  ")
			b.WriteString(r)
			b.WriteString("\n")
		}
	}
	return b.String()
}