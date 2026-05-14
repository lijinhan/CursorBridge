package agent

import (
	"context"
	"io"
	"strings"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func HandleStreamDiffReview(ctx context.Context, reqBody []byte, contentType string, rawWriter io.Writer, resolve AdapterResolver, selectedModel string) {
	req := &aiserverv1.GetDiffReviewRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		writeDiffReviewEndStreamError(rawWriter, "decode diff review: "+err.Error())
		return
	}
	target, ok := ResolveAdapterForModel(resolve.Resolve(), selectedModel)
	if !ok {
		writeDiffReviewEndStreamError(rawWriter, "no BYOK adapter configured")
		return
	}
	stream := pickProviderStreamer(target.ProviderType)
	prompt := buildDiffReviewPrompt(req)
	fastOpts := target.Opts
	fastOpts.ReasoningEffort = ""
	fastOpts.ServiceTier = ""
	if fastOpts.MaxOutputTokens <= 0 || fastOpts.MaxOutputTokens > 500 {
		fastOpts.MaxOutputTokens = 500
	}
	_, err := stream(ctx, target.BaseURL, target.APIKey, target.Model, []openAIMessage{
		textMessage("system", "You are a code reviewer. Review the provided diff and identify potential issues: bugs, security problems, performance concerns, or style issues. Be specific and concise."),
		textMessage("user", prompt),
	}, nil, fastOpts, func(chunk string, reasoning string, done bool) error {
		if chunk != "" {
			resp := &aiserverv1.StreamDiffReviewResponse{
				Response: &aiserverv1.StreamDiffReviewResponse_Text{Text: chunk},
			}
			body, _ := proto.Marshal(resp)
			if err := writeFrame(rawWriter, body, done); err != nil {
				return err
			}
			flushIfPossible(rawWriter)
		}
		return nil
	})
	if err != nil {
		writeDiffReviewEndStreamError(rawWriter, err.Error())
		return
	}
	_ = writeEndStream(rawWriter)
}

func buildDiffReviewPrompt(req *aiserverv1.GetDiffReviewRequest) string {
	var b strings.Builder
	b.WriteString("Review this code diff for potential issues:\n\n")
	for _, diff := range req.GetDiffs() {
		if path := diff.GetRelativeWorkspacePath(); path != "" {
			b.WriteString("File: ")
			b.WriteString(path)
			b.WriteString("\n")
		}
		for _, chunk := range diff.GetChunks() {
			var chunkText strings.Builder
			for _, old := range chunk.GetOldLines() {
				chunkText.WriteString("- ")
				chunkText.WriteString(old)
				chunkText.WriteString("\n")
			}
			for _, new := range chunk.GetNewLines() {
				chunkText.WriteString("+ ")
				chunkText.WriteString(new)
				chunkText.WriteString("\n")
			}
			if chunkText.Len() > 0 {
				b.WriteString(chunkText.String())
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func writeDiffReviewEndStreamError(w io.Writer, msg string) {
	resp := &aiserverv1.StreamDiffReviewResponse{
		Response: &aiserverv1.StreamDiffReviewResponse_Text{Text: "Error: " + msg},
	}
	body, _ := proto.Marshal(resp)
	_ = writeFrame(w, body, true)
}