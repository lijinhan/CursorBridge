package agent

import (
	"net/http"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func HandleGetMcpTools(reqBody []byte, contentType string) Result {
	req := &aiserverv1.GetMcpToolsRequest{}
	if err := decodeUnary(reqBody, contentType, req); err != nil {
		return errResult(http.StatusBadRequest, "decode get mcp tools: "+err.Error())
	}
	body, err := proto.Marshal(&aiserverv1.GetMcpToolsResponse{})
	if err != nil {
		return errResult(http.StatusInternalServerError, "marshal mcp tools response: "+err.Error())
	}
	return Result{Status: http.StatusOK, ContentType: "application/proto", Body: body}
}