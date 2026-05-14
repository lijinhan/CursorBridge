package mitm

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"testing"

	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------------------------
// isTelemetryPath
// ---------------------------------------------------------------------------

func TestIsTelemetryPath_PrefixMatches(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"Log", "/aiserver.v1.AiService/LogEvent", true},
		{"Record", "/aiserver.v1.AiService/RecordUsage", true},
		{"Report", "/aiserver.v1.AiService/ReportBug", true},
		{"Track", "/aiserver.v1.AiService/TrackFeature", true},
		{"Submit", "/aiserver.v1.AiService/SubmitFeedback", true},
		{"Send", "/aiserver.v1.AiService/SendMetrics", true},
		{"Ingest", "/aiserver.v1.AiService/IngestData", true},
		{"Collect", "/aiserver.v1.AiService/CollectStats", true},
		{"Upload", "/aiserver.v1.AiService/UploadCrash", true},
		{"Post", "/aiserver.v1.AiService/PostEvent", true},
		{"Capture", "/aiserver.v1.AiService/CaptureSnapshot", true},
		{"Emit", "/aiserver.v1.AiService/EmitSignal", true},
		{"Flush", "/aiserver.v1.AiService/FlushQueue", true},
		{"Batch", "/aiserver.v1.AiService/BatchUpload", true},
		{"Put", "/aiserver.v1.AiService/PutMetric", true},
		{"Store", "/aiserver.v1.AiService/StoreReport", true},
		{"CreateFeed", "/aiserver.v1.AiService/CreateFeedItem", true},
		{"GetSentry", "/aiserver.v1.AiService/GetSentryConfig", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTelemetryPath(tt.path); got != tt.want {
				t.Errorf("isTelemetryPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsTelemetryPath_ExactMatches(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"GetTelemetryConfig", "/aiserver.v1.AiService/GetTelemetryConfig", true},
		{"GetAnalyticsConfig", "/aiserver.v1.AiService/GetAnalyticsConfig", true},
		{"GetCrashReportConfig", "/aiserver.v1.AiService/GetCrashReportConfig", true},
		{"GetSentryDsn", "/aiserver.v1.AiService/GetSentryDsn", true},
		{"GetPostHogApiKey", "/aiserver.v1.AiService/GetPostHogApiKey", true},
		{"GetAmplitudeApiKey", "/aiserver.v1.AiService/GetAmplitudeApiKey", true},
		{"GetMixpanelApiKey", "/aiserver.v1.AiService/GetMixpanelApiKey", true},
		{"GetSegmentWriteKey", "/aiserver.v1.AiService/GetSegmentWriteKey", true},
		{"GetLaunchDarklyClientId", "/aiserver.v1.AiService/GetLaunchDarklyClientId", true},
		{"GetStatsigFeatureGates", "/aiserver.v1.AiService/GetStatsigFeatureGates", true},
		{"GetDynamicConfig", "/aiserver.v1.AiService/GetDynamicConfig", true},
		{"GetExperimentConfig", "/aiserver.v1.AiService/GetExperimentConfig", true},
		{"GetABTestConfig", "/aiserver.v1.AiService/GetABTestConfig", true},
		{"GetPrivacyConfig", "/aiserver.v1.AiService/GetPrivacyConfig", true},
		{"GetNotificationConfig", "/aiserver.v1.AiService/GetNotificationConfig", true},
		{"GetExtensionRecommendations", "/aiserver.v1.AiService/GetExtensionRecommendations", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTelemetryPath(tt.path); got != tt.want {
				t.Errorf("isTelemetryPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsTelemetryPath_NonTelemetry(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"GetMe", "/aiserver.v1.AiService/GetMe", false},
		{"GetEmail", "/aiserver.v1.AiService/GetEmail", false},
		{"AvailableModels", "/aiserver.v1.AiService/AvailableModels", false},
		{"DifferentService", "/aiserver.v1.NetworkService/IsConnected", false},
		{"EmptyPath", "", false},
		{"RootSlash", "/", false},
		{"OtherPrefix", "/aiserver.v1.BidiService/BidiAppend", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTelemetryPath(tt.path); got != tt.want {
				t.Errorf("isTelemetryPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isEmptyMockPath
// ---------------------------------------------------------------------------

func TestIsEmptyMockPath_DashboardService(t *testing.T) {
	if !isEmptyMockPath("/aiserver.v1.DashboardService/GetDashboard") {
		t.Error("expected DashboardService path to match")
	}
}

func TestIsEmptyMockPath_AnalyticsService(t *testing.T) {
	if !isEmptyMockPath("/aiserver.v1.AnalyticsService/GetStats") {
		t.Error("expected AnalyticsService path to match")
	}
}

func TestIsEmptyMockPath_BackgroundComposerService(t *testing.T) {
	if !isEmptyMockPath("/aiserver.v1.BackgroundComposerService/SomeMethod") {
		t.Error("expected BackgroundComposerService path to match")
	}
}

func TestIsEmptyMockPath_NonMatchingPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"AiService", "/aiserver.v1.AiService/GetMe", false},
		{"NetworkService", "/aiserver.v1.NetworkService/IsConnected", false},
		{"Empty", "", false},
		{"PartialMatch", "/aiserver.v1.Dashboard", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyMockPath(tt.path); got != tt.want {
				t.Errorf("isEmptyMockPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// decodeUnaryMsg
// ---------------------------------------------------------------------------

// buildConnectEnvelope constructs a Connect unary envelope:
//   byte 0: flags (0x01 = gzip compressed)
//   bytes 1-4: big-endian uint32 payload length
//   bytes 5+: payload
func buildConnectEnvelope(flags byte, payload []byte) []byte {
	n := len(payload)
	envelope := make([]byte, 5+n)
	envelope[0] = flags
	envelope[1] = byte(n >> 24)
	envelope[2] = byte(n >> 16)
	envelope[3] = byte(n >> 8)
	envelope[4] = byte(n)
	copy(envelope[5:], payload)
	return envelope
}

func TestDecodeUnaryMsg_PlainConnectEnvelope(t *testing.T) {
	msg := &aiserverv1.KnowledgeBaseAddRequest{
		Knowledge: "test-knowledge",
		Title:     "test-title",
	}
	protoBytes, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	envelope := buildConnectEnvelope(0x00, protoBytes)
	got := &aiserverv1.KnowledgeBaseAddRequest{}
	if err := decodeUnaryMsg(envelope, "application/connect+proto", got); err != nil {
		t.Fatalf("decodeUnaryMsg: %v", err)
	}
	if got.GetKnowledge() != "test-knowledge" {
		t.Errorf("knowledge = %q, want %q", got.GetKnowledge(), "test-knowledge")
	}
	if got.GetTitle() != "test-title" {
		t.Errorf("title = %q, want %q", got.GetTitle(), "test-title")
	}
}

func TestDecodeUnaryMsg_GzipConnectEnvelope(t *testing.T) {
	msg := &aiserverv1.KnowledgeBaseAddRequest{
		Knowledge: "gzip-knowledge",
		Title:     "gzip-title",
	}
	protoBytes, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(protoBytes); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	w.Close()

	envelope := buildConnectEnvelope(0x01, buf.Bytes())
	got := &aiserverv1.KnowledgeBaseAddRequest{}
	if err := decodeUnaryMsg(envelope, "application/connect+proto", got); err != nil {
		t.Fatalf("decodeUnaryMsg: %v", err)
	}
	if got.GetKnowledge() != "gzip-knowledge" {
		t.Errorf("knowledge = %q, want %q", got.GetKnowledge(), "gzip-knowledge")
	}
	if got.GetTitle() != "gzip-title" {
		t.Errorf("title = %q, want %q", got.GetTitle(), "gzip-title")
	}
}

func TestDecodeUnaryMsg_NonConnectContentType(t *testing.T) {
	msg := &aiserverv1.KnowledgeBaseAddRequest{
		Knowledge: "raw-knowledge",
	}
	protoBytes, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}

	got := &aiserverv1.KnowledgeBaseAddRequest{}
	if err := decodeUnaryMsg(protoBytes, "application/proto", got); err != nil {
		t.Fatalf("decodeUnaryMsg: %v", err)
	}
	if got.GetKnowledge() != "raw-knowledge" {
		t.Errorf("knowledge = %q, want %q", got.GetKnowledge(), "raw-knowledge")
	}
}

func TestDecodeUnaryMsg_ShortBody(t *testing.T) {
	got := &aiserverv1.KnowledgeBaseAddRequest{}
	// Body shorter than 5 bytes with connect content-type should still work:
	// the envelope check len(payload) >= 5 fails, so it falls through to
	// proto.Unmarshal on the raw body.
	if err := decodeUnaryMsg([]byte{0x00}, "application/connect+proto", got); err == nil {
		// proto.Unmarshal on garbage may or may not error; just ensure no panic.
	}
}

func TestDecodeUnaryMsg_EnvelopeLengthExceedsBody(t *testing.T) {
	// Declare a length of 1000 but only provide 2 bytes of payload.
	envelope := []byte{0x00, 0x00, 0x00, 0x03, 0xE8, 0xAA, 0xBB}
	got := &aiserverv1.KnowledgeBaseAddRequest{}
	// The length check len(payload) >= 5+length fails, so it falls through
	// to proto.Unmarshal on the full body — which will fail on garbage.
	// The important thing is no panic or index-out-of-range.
	_ = decodeUnaryMsg(envelope, "application/connect+proto", got)
}

// ---------------------------------------------------------------------------
// mockProto / mock404
// ---------------------------------------------------------------------------

func TestMockProto_EmptyBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/test", nil)
	resp := mockProto(req, nil)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if resp.Status != "200 OK" {
		t.Errorf("Status = %q, want %q", resp.Status, "200 OK")
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/proto" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/proto")
	}
	if resp.ContentLength != 0 {
		t.Errorf("ContentLength = %d, want 0", resp.ContentLength)
	}
	if resp.Body == nil {
		t.Error("Body should not be nil")
	}
}

func TestMockProto_WithBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/test", nil)
	body := []byte{0x01, 0x02, 0x03}
	resp := mockProto(req, body)

	if resp.ContentLength != int64(len(body)) {
		t.Errorf("ContentLength = %d, want %d", resp.ContentLength, len(body))
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, body) {
		t.Errorf("Body = %v, want %v", got, body)
	}
}

func TestMockProto_PreservesRequestProto(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/test", nil)
	req.ProtoMajor = 2
	req.ProtoMinor = 0
	resp := mockProto(req, nil)

	if resp.ProtoMajor != 2 || resp.ProtoMinor != 0 {
		t.Errorf("Proto = %d.%d, want 2.0", resp.ProtoMajor, resp.ProtoMinor)
	}
	if resp.Request != req {
		t.Error("Request field should point back to original request")
	}
}

func TestMock404(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/test", nil)
	resp := mock404(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	if resp.Status != "404 Not Found" {
		t.Errorf("Status = %q, want %q", resp.Status, "404 Not Found")
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/plain; charset=utf-8")
	}
	xcto := resp.Header.Get("X-Content-Type-Options")
	if xcto != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", xcto, "nosniff")
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("404 body should not be empty")
	}
}

func TestMock404_PreservesRequestProto(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api2.cursor.sh/test", nil)
	req.ProtoMajor = 1
	req.ProtoMinor = 1
	resp := mock404(req)

	if resp.ProtoMajor != 1 || resp.ProtoMinor != 1 {
		t.Errorf("Proto = %d.%d, want 1.1", resp.ProtoMajor, resp.ProtoMinor)
	}
	if resp.Request != req {
		t.Error("Request field should point back to original request")
	}
}

// ---------------------------------------------------------------------------
// readDecodedBody
// ---------------------------------------------------------------------------

func TestReadDecodedBody_Plain(t *testing.T) {
	content := []byte("hello world")
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", bytes.NewReader(content))
	req.Header.Set("Content-Encoding", "identity")

	got, err := readDecodedBody(req)
	if err != nil {
		t.Fatalf("readDecodedBody: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("body = %q, want %q", got, content)
	}
}

func TestReadDecodedBody_Gzip(t *testing.T) {
	content := []byte("gzip compressed body")
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write(content)
	w.Close()

	req, _ := http.NewRequest(http.MethodPost, "https://example.com", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")

	got, err := readDecodedBody(req)
	if err != nil {
		t.Fatalf("readDecodedBody: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("body = %q, want %q", got, content)
	}
}

func TestReadDecodedBody_XGzip(t *testing.T) {
	content := []byte("x-gzip content")
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write(content)
	w.Close()

	req, _ := http.NewRequest(http.MethodPost, "https://example.com", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Encoding", "x-gzip")

	got, err := readDecodedBody(req)
	if err != nil {
		t.Fatalf("readDecodedBody: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("body = %q, want %q", got, content)
	}
}

func TestReadDecodedBody_NoEncoding(t *testing.T) {
	content := []byte("no encoding")
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", bytes.NewReader(content))

	got, err := readDecodedBody(req)
	if err != nil {
		t.Fatalf("readDecodedBody: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("body = %q, want %q", got, content)
	}
}
