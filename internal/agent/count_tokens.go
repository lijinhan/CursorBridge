package agent

import (
	"net/http"
	"strings"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func HandleCountTokens(reqBody []byte, contentType string) Result {
	req := &aiserverv1.CountTokensRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		return errResult(http.StatusBadRequest, "decode count tokens: "+err.Error())
	}
	var total int32
	var details []*aiserverv1.ContextItemTokenDetail
	for _, item := range req.GetContextItems() {
		text := extractContextItemText(item)
		count := estimateTokenCount(text)
		total += count
		details = append(details, &aiserverv1.ContextItemTokenDetail{
			RelativeWorkspacePath: extractContextItemPath(item),
			Count:                 count,
			LineCount:             int32(strings.Count(text, "\n") + 1),
		})
	}
	body, err := proto.Marshal(&aiserverv1.CountTokensResponse{Count: total, TokenDetails: details})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal count tokens response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}

func estimateTokenCount(text string) int32 {
	if text == "" {
		return 0
	}
	return int32(len(text) / 4 + 1)
}

func extractContextItemText(item *aiserverv1.ContextItem) string {
	if item == nil {
		return ""
	}
	switch c := item.Item.(type) {
	case *aiserverv1.ContextItem_FileChunk_:
		return c.FileChunk.GetChunkContents()
	case *aiserverv1.ContextItem_OutlineChunk_:
		return c.OutlineChunk.GetContents()
	case *aiserverv1.ContextItem_CmdKSelection_:
		return strings.Join(c.CmdKSelection.GetLines(), "\n")
	case *aiserverv1.ContextItem_CmdKImmediateContext_:
		var b strings.Builder
		for _, line := range c.CmdKImmediateContext.GetLines() {
			b.WriteString(line.GetLine())
			b.WriteString("\n")
		}
		return b.String()
	case *aiserverv1.ContextItem_CmdKQuery_:
		return c.CmdKQuery.GetQuery()
	default:
		return ""
	}
}

func extractContextItemPath(item *aiserverv1.ContextItem) string {
	if item == nil {
		return ""
	}
	switch c := item.Item.(type) {
	case *aiserverv1.ContextItem_FileChunk_:
		return c.FileChunk.GetRelativeWorkspacePath()
	case *aiserverv1.ContextItem_OutlineChunk_:
		return c.OutlineChunk.GetRelativeWorkspacePath()
	case *aiserverv1.ContextItem_CmdKImmediateContext_:
		return c.CmdKImmediateContext.GetRelativeWorkspacePath()
	default:
		return ""
	}
}