package mitm

import (
	"net/http"
	"testing"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

func TestHandleSyntheticPath_IsConnected(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/aiserver.v1.NetworkService/IsConnected", nil)
	resp := handleSyntheticPath(req, "/aiserver.v1.NetworkService/IsConnected")
	if resp == nil {
		t.Fatal("expected non-nil response for IsConnected")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var msg aiserverv1.IsConnectedResponse
	if err := proto.Unmarshal(readBody(resp), &msg); err != nil {
		t.Fatalf("unmarshal IsConnectedResponse: %v", err)
	}
}

func TestHandleSyntheticPath_GetMe(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/aiserver.v1.AiService/GetMe", nil)
	resp := handleSyntheticPath(req, "/aiserver.v1.AiService/GetMe")
	if resp == nil {
		t.Fatal("expected non-nil response for GetMe")
	}
	var msg aiserverv1.GetMeResponse
	if err := proto.Unmarshal(readBody(resp), &msg); err != nil {
		t.Fatalf("unmarshal GetMeResponse: %v", err)
	}
	if msg.GetAuthId() != syntheticFakeAuthID {
		t.Errorf("expected authId %q, got %q", syntheticFakeAuthID, msg.GetAuthId())
	}
	if msg.GetEmail() != syntheticFakeEmail {
		t.Errorf("expected email %q, got %q", syntheticFakeEmail, msg.GetEmail())
	}
}

func TestHandleSyntheticPath_GetEmail(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/aiserver.v1.AiService/GetEmail", nil)
	resp := handleSyntheticPath(req, "/aiserver.v1.AiService/GetEmail")
	if resp == nil {
		t.Fatal("expected non-nil response for GetEmail")
	}
	var msg aiserverv1.GetEmailResponse
	if err := proto.Unmarshal(readBody(resp), &msg); err != nil {
		t.Fatalf("unmarshal GetEmailResponse: %v", err)
	}
	if msg.GetEmail() != syntheticFakeEmail {
		t.Errorf("expected email %q, got %q", syntheticFakeEmail, msg.GetEmail())
	}
}

func TestHandleSyntheticPath_GetUserInfo(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/aiserver.v1.AiService/GetUserInfo", nil)
	resp := handleSyntheticPath(req, "/aiserver.v1.AiService/GetUserInfo")
	if resp == nil {
		t.Fatal("expected non-nil response for GetUserInfo")
	}
	var msg aiserverv1.GetUserInfoResponse
	if err := proto.Unmarshal(readBody(resp), &msg); err != nil {
		t.Fatalf("unmarshal GetUserInfoResponse: %v", err)
	}
	if msg.GetUserId() != syntheticFakeAuthID {
		t.Errorf("expected userId %q, got %q", syntheticFakeAuthID, msg.GetUserId())
	}
}

func TestHandleSyntheticPath_GetUserMeta(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/aiserver.v1.AiService/GetUserMeta", nil)
	resp := handleSyntheticPath(req, "/aiserver.v1.AiService/GetUserMeta")
	if resp == nil {
		t.Fatal("expected non-nil response for GetUserMeta")
	}
	var msg aiserverv1.GetUserMetaResponse
	if err := proto.Unmarshal(readBody(resp), &msg); err != nil {
		t.Fatalf("unmarshal GetUserMetaResponse: %v", err)
	}
	if msg.GetEmail() != syntheticFakeEmail {
		t.Errorf("expected email %q, got %q", syntheticFakeEmail, msg.GetEmail())
	}
}

func TestHandleSyntheticPath_Unknown(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/aiserver.v1.AiService/Unknown", nil)
	resp := handleSyntheticPath(req, "/aiserver.v1.AiService/Unknown")
	if resp != nil {
		t.Fatal("expected nil response for unknown path")
	}
}

func readBody(resp *http.Response) []byte {
	if resp.Body == nil {
		return nil
	}
	defer resp.Body.Close()
	data := make([]byte, resp.ContentLength)
	_, _ = resp.Body.Read(data)
	return data
}