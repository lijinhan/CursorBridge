package agent

import (
	"context"
	"net/http"
	"strings"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func HandleGetChatSuggestions(ctx context.Context, reqBody []byte, contentType string, resolve AdapterResolver, selectedModel string) Result {
	req := &aiserverv1.GetChatSuggestionsRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		return errResult(http.StatusBadRequest, "decode chat suggestions: "+err.Error())
	}
	target, ok := ResolveAdapterForModel(resolve.Resolve(), selectedModel)
	if !ok {
		return errResult(http.StatusBadRequest, "no BYOK adapter configured")
	}
	stream := pickProviderStreamer(target.ProviderType)
	suggestions, err := generateChatSuggestions(ctx, req, target.BaseURL, target.APIKey, target.Model, target.Opts, stream)
	if err != nil {
		return errResult(http.StatusBadGateway, "generate chat suggestions: "+err.Error())
	}
	body, err := proto.Marshal(&aiserverv1.GetChatSuggestionsResponse{Suggestions: suggestions})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal chat suggestions response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}

func generateChatSuggestions(ctx context.Context, req *aiserverv1.GetChatSuggestionsRequest, baseURL, apiKey, model string, opts AdapterOpts, stream providerStreamer) ([]*aiserverv1.ChatSuggestionItem, error) {
	prompt := buildChatSuggestionsPrompt(req)
	fastOpts := opts
	fastOpts.ReasoningEffort = ""
	fastOpts.ServiceTier = ""
	if fastOpts.MaxOutputTokens <= 0 || fastOpts.MaxOutputTokens > 300 {
		fastOpts.MaxOutputTokens = 300
	}
	result, err := collectSingleResponse(ctx, stream, baseURL, apiKey, model, fastOpts, []openAIMessage{
		textMessage("system", "You are a coding assistant that suggests what a user might want to ask next. Given recent chat context, suggest 3 short follow-up questions or tasks. Return one suggestion per line, no numbering, no quotes, no extra text."),
		textMessage("user", prompt),
	})
	if err != nil {
		return nil, err
	}
	lines := strings.Split(result, "\n")
	var suggestions []*aiserverv1.ChatSuggestionItem
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		suggestions = append(suggestions, &aiserverv1.ChatSuggestionItem{
			Text: text,
		})
		if len(suggestions) >= 5 {
			break
		}
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, &aiserverv1.ChatSuggestionItem{Text: "How can I improve this code?"})
	}
	return suggestions, nil
}

func buildChatSuggestionsPrompt(req *aiserverv1.GetChatSuggestionsRequest) string {
	var b strings.Builder
	b.WriteString("Suggest follow-up questions based on recent chats:\n\n")
	for i, chat := range req.GetRecentChats() {
		if i > 5 {
			break
		}
		title := chat.GetTitle()
		if title == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(title)
		b.WriteString("\n")
	}
	return b.String()
}