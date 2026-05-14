package agent

import (
	"context"
	"net/http"
	"strings"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func HandleLintExplanation(ctx context.Context, reqBody []byte, contentType string, resolve AdapterResolver, selectedModel string) Result {
	req := &aiserverv1.LintExplanationRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		return errResult(http.StatusBadRequest, "decode lint explanation: "+err.Error())
	}
	target, ok := ResolveAdapterForModel(resolve.Resolve(), selectedModel)
	if !ok {
		return errResult(http.StatusBadRequest, "no BYOK adapter configured")
	}
	stream := pickProviderStreamer(target.ProviderType)
	explanation, err := generateLintExplanation(ctx, req, target.BaseURL, target.APIKey, target.Model, target.Opts, stream)
	if err != nil {
		return errResult(http.StatusBadGateway, "generate lint explanation: "+err.Error())
	}
	body, err := proto.Marshal(&aiserverv1.LintExplanationResponse{Explanation: explanation})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal lint explanation response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}

func generateLintExplanation(ctx context.Context, req *aiserverv1.LintExplanationRequest, baseURL, apiKey, model string, opts AdapterOpts, stream providerStreamer) (string, error) {
	prompt := buildLintExplanationPrompt(req)
	fastOpts := opts
	fastOpts.ReasoningEffort = ""
	fastOpts.ServiceTier = ""
	if fastOpts.MaxOutputTokens <= 0 || fastOpts.MaxOutputTokens > 300 {
		fastOpts.MaxOutputTokens = 300
	}
	result, err := collectSingleResponse(ctx, stream, baseURL, apiKey, model, fastOpts, []openAIMessage{
		textMessage("system", "Explain the lint error in the given code context and suggest a fix. Be concise: explain what the error means and how to fix it. Return only the explanation text."),
		textMessage("user", prompt),
	})
	if err != nil {
		return "", err
	}
	explanation := strings.TrimSpace(result)
	if explanation == "" {
		explanation = "Unable to explain this lint error."
	}
	return explanation, nil
}

func buildLintExplanationPrompt(req *aiserverv1.LintExplanationRequest) string {
	var b strings.Builder
	b.WriteString("Explain this lint error and suggest a fix:\n\n")
	if path := req.GetRelativeFilePath(); path != "" {
		b.WriteString("File: ")
		b.WriteString(path)
		b.WriteString("\n")
	}
	if chunk := req.GetChunk(); chunk != nil {
		b.WriteString("Code context (starting at line ")
		b.WriteString(strings.TrimSpace(chunk.GetChunkContents()))
		b.WriteString("\n")
	}
	if selection := req.GetLineSelection(); selection != "" {
		b.WriteString("Selected text: ")
		b.WriteString(selection)
		b.WriteString("\n")
	}
	if alt := req.GetLikelyAlternateToken(); alt != "" {
		b.WriteString("Likely fix: ")
		b.WriteString(alt)
		b.WriteString("\n")
	}
	return b.String()
}