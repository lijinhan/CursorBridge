package agent

import (
	"net/http"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func HandleGetConversationSummary(reqBody []byte, contentType string) Result {
	req := &aiserverv1.GetConversationSummaryRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		return errResult(http.StatusBadRequest, "decode conversation summary: "+err.Error())
	}
	var summaries []*aiserverv1.ConversationSummaryData
	for _, id := range req.GetConversationIds() {
		summaries = append(summaries, &aiserverv1.ConversationSummaryData{
			ConversationId: id,
		})
	}
	body, err := proto.Marshal(&aiserverv1.GetConversationSummaryResponse{Summaries: summaries})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal conversation summary response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}