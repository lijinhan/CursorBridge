package agent

import (
	"context"
	"net/http"
	"strings"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

// HandleGetChatTitle generates a short title for a conversation using the
// configured BYOK adapter. It extracts the user's messages from the request,
// builds a summarization prompt, and returns a GetChatTitleResponse proto.
func HandleGetChatTitle(ctx context.Context, reqBody []byte, contentType string, resolve AdapterResolver, selectedModel string) Result {
	req := &aiserverv1.GetChatTitleRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		return errResult(http.StatusBadRequest, "decode get chat title: "+err.Error())
	}
	target, ok := ResolveAdapterForModel(resolve.Resolve(), selectedModel)
	if !ok {
		return errResult(http.StatusBadRequest, "no BYOK adapter configured")
	}
	stream := pickProviderStreamer(target.ProviderType)
	title, err := generateChatTitle(ctx, req, target.BaseURL, target.APIKey, target.Model, target.Opts, stream)
	if err != nil {
		return errResult(http.StatusBadGateway, "generate chat title: "+err.Error())
	}
	body, err := proto.Marshal(&aiserverv1.GetChatTitleResponse{Title: title})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal chat title response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}

func generateChatTitle(ctx context.Context, req *aiserverv1.GetChatTitleRequest, baseURL, apiKey, model string, opts AdapterOpts, stream providerStreamer) (string, error) {
	prompt := buildChatTitlePrompt(req)
	fastOpts := opts
	fastOpts.ReasoningEffort = ""
	fastOpts.ServiceTier = ""
	if fastOpts.MaxOutputTokens <= 0 || fastOpts.MaxOutputTokens > 80 {
		fastOpts.MaxOutputTokens = 80
	}
	result, err := collectSingleResponse(ctx, stream, baseURL, apiKey, model, fastOpts, []openAIMessage{
		textMessage("system", "Generate a very short title (3-6 words) for the given conversation. Return only the title text, nothing else. No quotes, no punctuation at the end."),
		textMessage("user", prompt),
	})
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(result)
	if len(title) > 80 {
		title = title[:77] + "..."
	}
	if title == "" {
		title = "New Chat"
	}
	return title, nil
}

func buildChatTitlePrompt(req *aiserverv1.GetChatTitleRequest) string {
	var b strings.Builder
	b.WriteString("Generate a concise title for this conversation:\n\n")
	for i, msg := range req.GetConversation() {
		if i > 5 {
			b.WriteString("... (more messages)\n")
			break
		}
		text := strings.TrimSpace(msg.GetText())
		if text == "" {
			continue
		}
		if len(text) > 500 {
			text = text[:497] + "..."
		}
		switch msg.GetType() {
		case 1:
			b.WriteString("User: ")
		case 2:
			b.WriteString("Assistant: ")
		default:
			b.WriteString("Message: ")
		}
		b.WriteString(text)
		b.WriteString("\n")
	}
	return b.String()
}