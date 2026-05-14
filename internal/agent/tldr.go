package agent

import (
	"context"
	"net/http"
	"strings"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func HandleGenerateTldr(ctx context.Context, reqBody []byte, contentType string, resolve AdapterResolver, selectedModel string) Result {
	req := &aiserverv1.GenerateTldrRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		return errResult(http.StatusBadRequest, "decode generate tldr: "+err.Error())
	}
	target, ok := ResolveAdapterForModel(resolve.Resolve(), selectedModel)
	if !ok {
		return errResult(http.StatusBadRequest, "no BYOK adapter configured")
	}
	stream := pickProviderStreamer(target.ProviderType)
	summary, all, err := generateTldr(ctx, req, target.BaseURL, target.APIKey, target.Model, target.Opts, stream)
	if err != nil {
		return errResult(http.StatusBadGateway, "generate tldr: "+err.Error())
	}
	body, err := proto.Marshal(&aiserverv1.GenerateTldrResponse{Summary: summary, All: all})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal tldr response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}

func generateTldr(ctx context.Context, req *aiserverv1.GenerateTldrRequest, baseURL, apiKey, model string, opts AdapterOpts, stream providerStreamer) (string, string, error) {
	text := req.GetText()
	if strings.TrimSpace(text) == "" {
		return "", "", nil
	}
	fastOpts := opts
	fastOpts.ReasoningEffort = ""
	fastOpts.ServiceTier = ""
	if fastOpts.MaxOutputTokens <= 0 || fastOpts.MaxOutputTokens > 200 {
		fastOpts.MaxOutputTokens = 200
	}
	result, err := collectSingleResponse(ctx, stream, baseURL, apiKey, model, fastOpts, []openAIMessage{
		textMessage("system", "Generate a TL;DR (too long; didn't read) summary. First line: a one-sentence summary. Then 2-3 bullet points for key takeaways. Return only the summary."),
		textMessage("user", text),
	})
	if err != nil {
		return "", "", err
	}
	summary := strings.TrimSpace(result)
	if summary == "" {
		summary = "No content to summarize."
	}
	return summary, summary, nil
}