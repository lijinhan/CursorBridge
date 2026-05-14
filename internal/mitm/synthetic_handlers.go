package mitm

import (
	"bytes"
	"io"
	"net/http"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

const (
	syntheticFakeEmail     = "cursor@ai.com"
	syntheticFakeFirstName = "Cursor"
	syntheticFakeLastName  = "AI"
	syntheticFakeAuthID    = "fake-cursor-local-user"
)

// syntheticPaths maps api2 paths that need structured (non-empty) proto
// responses rather than the generic mock-200 empty body. These are paths
// where Cursor's UI reads specific fields from the response to render
// connection status, user info, etc.
var syntheticPaths = map[string]func(req *http.Request) []byte{
	"/aiserver.v1.NetworkService/IsConnected": handleIsConnected,
	"/aiserver.v1.AiService/GetMe":            handleGetMe,
	"/aiserver.v1.AiService/GetEmail":         handleGetEmail,
	"/aiserver.v1.AiService/GetUserInfo":      handleGetUserInfo,
	"/aiserver.v1.AiService/GetUserMeta":      handleGetUserMeta,
}

// handleSyntheticPath checks whether the given path has a structured mock
// response and returns it. Returns nil if the path is not a synthetic path.
func handleSyntheticPath(req *http.Request, path string) *http.Response {
	fn, ok := syntheticPaths[path]
	if !ok {
		return nil
	}
	body := fn(req)
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Header:        http.Header{"Content-Type": {"application/proto"}},
		Request:       req,
	}
}

func handleIsConnected(_ *http.Request) []byte {
	resp := &aiserverv1.IsConnectedResponse{}
	out, _ := proto.Marshal(resp)
	return out
}

func handleGetMe(_ *http.Request) []byte {
	email := syntheticFakeEmail
	firstName := syntheticFakeFirstName
	lastName := syntheticFakeLastName
	resp := &aiserverv1.GetMeResponse{
		AuthId:    syntheticFakeAuthID,
		UserId:    1,
		Email:     &email,
		FirstName: &firstName,
		LastName:  &lastName,
	}
	out, _ := proto.Marshal(resp)
	return out
}

func handleGetEmail(_ *http.Request) []byte {
	resp := &aiserverv1.GetEmailResponse{
		Email:      syntheticFakeEmail,
		SignUpType: aiserverv1.GetEmailResponse_SIGN_UP_TYPE_AUTH_0,
	}
	out, _ := proto.Marshal(resp)
	return out
}

func handleGetUserInfo(_ *http.Request) []byte {
	resp := &aiserverv1.GetUserInfoResponse{
		UserId: syntheticFakeAuthID,
	}
	out, _ := proto.Marshal(resp)
	return out
}

func handleGetUserMeta(_ *http.Request) []byte {
	resp := &aiserverv1.GetUserMetaResponse{
		Email:      syntheticFakeEmail,
		SignUpType: aiserverv1.GetEmailResponse_SIGN_UP_TYPE_AUTH_0,
		UserId:     1,
	}
	out, _ := proto.Marshal(resp)
	return out
}