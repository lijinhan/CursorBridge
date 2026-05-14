package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func HandleGetCompletion(ctx context.Context, reqBody []byte, contentType string, resolve AdapterResolver, selectedModel string) Result {
	req := &aiserverv1.GetCompletionRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		return errResult(http.StatusBadRequest, "decode completion: "+err.Error())
	}
	target, ok := ResolveAdapterForModel(resolve.Resolve(), selectedModel)
	if !ok {
		return errResult(http.StatusBadRequest, "no BYOK adapter configured")
	}
	stream := pickProviderStreamer(target.ProviderType)
	completion, score, err := generateCompletion(ctx, req, target, stream)
	if err != nil {
		return errResult(http.StatusBadGateway, "generate completion: "+err.Error())
	}
	body, err := proto.Marshal(&aiserverv1.GetCompletionResponse{Completion: completion, Score: score})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal completion response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}

func generateCompletion(ctx context.Context, req *aiserverv1.GetCompletionRequest, target AdapterTarget, stream providerStreamer) (string, float32, error) {
	prompt := buildCompletionPrompt(req)
	fastOpts := target.Opts
	fastOpts.ReasoningEffort = ""
	fastOpts.ServiceTier = ""
	if fastOpts.MaxOutputTokens <= 0 || fastOpts.MaxOutputTokens > 60 {
		fastOpts.MaxOutputTokens = 60
	}
	result, err := collectSingleResponse(ctx, stream, target.BaseURL, target.APIKey, target.Model, fastOpts, []openAIMessage{
		textMessage("system", "You are a code completion engine. Given code context with a cursor position, output ONLY the code that should be inserted at the cursor. No explanations, no markdown, no comments — just the raw code snippet to complete the line/block."),
		textMessage("user", prompt),
	})
	if err != nil {
		return "", 0, err
	}
	completion := strings.TrimSpace(result)
	score := float32(0.8)
	if completion == "" {
		score = 0
	}
	return completion, score, nil
}

func buildCompletionPrompt(req *aiserverv1.GetCompletionRequest) string {
	var b strings.Builder
	if fi := req.GetFileIdentifier(); fi != nil {
		if path := fi.GetRelativePath(); path != "" {
			b.WriteString("File: ")
			b.WriteString(path)
			b.WriteString("\n")
		}
		if lang := fi.GetLanguageId(); lang != "" {
			b.WriteString("Language: ")
			b.WriteString(lang)
			b.WriteString("\n")
		}
	}
	if pos := req.GetCursorPosition(); pos != nil {
		b.WriteString("Cursor position: line ")
		b.WriteString(int32Str(pos.GetLine()))
		b.WriteString(", column ")
		b.WriteString(int32Str(pos.GetColumn()))
		b.WriteString("\n")
	}
	if sl := req.GetSurroundingLines(); sl != nil {
		lines := sl.GetLines()
		startLine := sl.GetStartLine()
		cursorLine := req.GetCursorPosition().GetLine()
		b.WriteString("Code context:\n")
		for i, line := range lines {
			lineNum := startLine + int32(i)
			prefix := "  "
			if lineNum == cursorLine {
				prefix = ">>"
			}
			b.WriteString(prefix)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	if ec := req.GetExplicitContext(); ec != nil {
		if ctx := strings.TrimSpace(ec.GetContext()); ctx != "" {
			b.WriteString("\nContext: ")
			b.WriteString(ctx)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func int32Str(v int32) string {
	return fmt.Sprintf("%d", v)
}