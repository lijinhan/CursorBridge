package agent

import (
	"context"
	"net/http"
	"strings"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func HandleGetChatTitle(ctx context.Context, reqBody []byte, contentType string, resolve AdapterResolver, selectedModel string) Result {
	req := &aiserverv1.GetChatTitleRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		return errResult(http.StatusBadRequest, "decode chat title: "+err.Error())
	}
	target, ok := ResolveAdapterForModel(resolve.Resolve(), selectedModel)
	if !ok {
		return errResult(http.StatusBadRequest, "no BYOK adapter configured")
	}
	stream := pickProviderStreamer(target.ProviderType)
	title, err := generateChatTitle(ctx, req, target, stream)
	if err != nil {
		return errResult(http.StatusBadGateway, "generate chat title: "+err.Error())
	}
	body, err := proto.Marshal(&aiserverv1.GetChatTitleResponse{Title: title})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal chat title response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}

func generateChatTitle(ctx context.Context, req *aiserverv1.GetChatTitleRequest, target AdapterTarget, stream providerStreamer) (string, error) {
	prompt := buildChatTitlePrompt(req)
	fastOpts := target.Opts
	fastOpts.ReasoningEffort = ""
	fastOpts.ServiceTier = ""
	if fastOpts.MaxOutputTokens <= 0 || fastOpts.MaxOutputTokens > 80 {
		fastOpts.MaxOutputTokens = 80
	}
	result, err := collectSingleResponse(ctx, stream, target.BaseURL, target.APIKey, target.Model, fastOpts, []openAIMessage{
		textMessage("system", "Generate a very short title (3-6 words) that summarizes the conversation. Return only the title text, no quotes, no punctuation at the end."),
		textMessage("user", prompt),
	})
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(result)
	if len(title) > 100 {
		title = title[:100]
	}
	return title, nil
}

func buildChatTitlePrompt(req *aiserverv1.GetChatTitleRequest) string {
	var b strings.Builder
	b.WriteString("Generate a short title for this conversation:\n\n")
	for _, msg := range req.GetConversation() {
		switch msg.GetType() {
		case aiserverv1.ConversationMessage_MESSAGE_TYPE_HUMAN:
			b.WriteString("User: ")
		case aiserverv1.ConversationMessage_MESSAGE_TYPE_AI:
			b.WriteString("Assistant: ")
		default:
			continue
		}
		if text := msg.GetText(); text != "" {
			if len(text) > 500 {
				text = text[:500] + "..."
			}
			b.WriteString(text)
		}
		b.WriteString("\n")
	}
	return b.String()
}
