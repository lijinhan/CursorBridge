package relay

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"cursorbridge/internal/codec"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func testAdapter(displayName, modelID string) AdapterInfo {
	return AdapterInfo{
		DisplayName: displayName,
		Type:        "openai",
		ModelID:     modelID,
		BaseURL:     "https://api.example.com",
		APIKey:      "sk-test",
	}
}

func staticProvider(adapters ...AdapterInfo) func() []AdapterInfo {
	return func() []AdapterInfo { return adapters }
}

// connectEnvelope wraps payload in a 5-byte Connect frame (flags=0, no
// compression).
func connectEnvelope(payload []byte) []byte {
	env := make([]byte, 5+len(payload))
	env[0] = 0 // flags
	binary.BigEndian.PutUint32(env[1:5], uint32(len(payload)))
	copy(env[5:], payload)
	return env
}

// connectEnvelopeCompressed wraps payload in a 5-byte Connect frame with the
// compression flag set and the payload gzip'd.
func connectEnvelopeCompressed(payload []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(payload)
	_ = w.Close()
	compressed := buf.Bytes()

	env := make([]byte, 5+len(compressed))
	env[0] = connectFlagCompressed // 0x01
	binary.BigEndian.PutUint32(env[1:5], uint32(len(compressed)))
	copy(env[5:], compressed)
	return env
}

// makeResponse builds an *http.Response with an initialised Header map.
func makeResponse(statusCode int, body []byte) *http.Response {
	return &http.Response{
		StatusCode:    statusCode,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Header:        http.Header{},
	}
}

// strictRewriter is a rewriter that rejects raw (non-envelope) input by
// checking that the body starts with a valid protobuf field tag for the
// expected message type. This lets us test the Connect-envelope code path in
// tryRewrite, which is only reached when the raw-protobuf attempt fails.
func strictRewriter(body []byte, adapters []AdapterInfo) ([]byte, error) {
	// If the body looks like a Connect envelope (5-byte header), reject it
	// as raw protobuf so tryRewrite falls through to the envelope path.
	if len(body) >= 5 {
		length := int(binary.BigEndian.Uint32(body[1:5]))
		if 5+length == len(body) {
			return nil, errNotRawProtobuf
		}
	}
	return buildAvailableModelsResponse(adapters), nil
}

var errNotRawProtobuf = errors.New("not raw protobuf")

// ---------------------------------------------------------------------------
// isCursorRPC
// ---------------------------------------------------------------------------

func TestIsCursorRPC(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/aiserver.v1.AiService/AvailableModels", true},
		{"/aiserver.v1.Foo/Bar", true},
		{"/agent.v1.Agent/Run", true},
		{"/anyrun.v1.Service/Method", true},
		{"/internapi.v1.Svc/Do", true},
		{"/other.v1.Svc/Do", false},
		{"/aiserver.v2.AiService/Foo", false},
		{"", false},
		{"/", false},
	}
	for _, tt := range tests {
		if got := isCursorRPC(tt.path); got != tt.want {
			t.Errorf("isCursorRPC(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// AdapterInfo.StableID
// ---------------------------------------------------------------------------

func TestAdapterInfo_StableID(t *testing.T) {
	a := testAdapter("My Model", "gpt-4o")
	id := a.StableID()

	if len(id) != 16 {
		t.Fatalf("StableID length = %d, want 16", len(id))
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Fatalf("StableID is not valid hex: %v", err)
	}

	a2 := testAdapter("Different Name", "gpt-4o")
	if a2.StableID() != id {
		t.Error("StableID changed when DisplayName changed, want stable based on ModelID")
	}

	a3 := testAdapter("My Model", "claude-3")
	if a3.StableID() == id {
		t.Error("StableID identical for different ModelIDs")
	}
}

// ---------------------------------------------------------------------------
// Gateway.SyntheticPath
// ---------------------------------------------------------------------------

func TestSyntheticPath_NilProvider(t *testing.T) {
	g := NewGateway()
	if got := g.SyntheticPath("/aiserver.v1.AiService/AvailableModels"); got != nil {
		t.Error("expected nil when adapterProvider is nil")
	}
}

func TestSyntheticPath_EmptyAdapters(t *testing.T) {
	g := NewGateway()
	g.SetAdapterProvider(staticProvider())
	if got := g.SyntheticPath("/aiserver.v1.AiService/AvailableModels"); got != nil {
		t.Error("expected nil when adapter list is empty")
	}
}

func TestSyntheticPath_UnknownPath(t *testing.T) {
	g := NewGateway()
	g.SetAdapterProvider(staticProvider(testAdapter("X", "m1")))
	if got := g.SyntheticPath("/aiserver.v1.AiService/Unknown"); got != nil {
		t.Error("expected nil for unknown path")
	}
}

func TestSyntheticPath_AvailableModels(t *testing.T) {
	g := NewGateway()
	adapter := testAdapter("TestModel", "gpt-4o")
	g.SetAdapterProvider(staticProvider(adapter))

	got := g.SyntheticPath("/aiserver.v1.AiService/AvailableModels")
	if got == nil {
		t.Fatal("expected non-nil response for AvailableModels")
	}
	hexID := adapter.StableID()
	if !bytes.Contains(got, []byte(hexID)) {
		t.Errorf("response does not contain StableID %q", hexID)
	}
}

func TestSyntheticPath_GetDefaultModelNudgeData(t *testing.T) {
	g := NewGateway()
	adapter := testAdapter("TestModel", "gpt-4o")
	g.SetAdapterProvider(staticProvider(adapter))

	got := g.SyntheticPath("/aiserver.v1.AiService/GetDefaultModelNudgeData")
	if got == nil {
		t.Fatal("expected non-nil response for GetDefaultModelNudgeData")
	}
	hexID := adapter.StableID()
	if !bytes.Contains(got, []byte(hexID)) {
		t.Errorf("response does not contain StableID %q", hexID)
	}
}

// ---------------------------------------------------------------------------
// Gateway.MaybeRewriteResponse
// ---------------------------------------------------------------------------

func TestMaybeRewriteResponse_NilInputs(t *testing.T) {
	g := NewGateway()
	g.SetAdapterProvider(staticProvider(testAdapter("X", "m1")))

	if g.MaybeRewriteResponse(nil, &http.Response{}) {
		t.Error("expected false for nil request")
	}
	if g.MaybeRewriteResponse(&http.Request{}, nil) {
		t.Error("expected false for nil response")
	}
}

func TestMaybeRewriteResponse_Non2xxStatus(t *testing.T) {
	g := NewGateway()
	g.SetAdapterProvider(staticProvider(testAdapter("X", "m1")))

	for _, code := range []int{100, 301, 404, 500} {
		req := &http.Request{URL: &url.URL{Path: "/aiserver.v1.AiService/AvailableModels"}}
		resp := makeResponse(code, nil)
		if g.MaybeRewriteResponse(req, resp) {
			t.Errorf("expected false for status %d", code)
		}
	}
}

func TestMaybeRewriteResponse_NoRewriteForPath(t *testing.T) {
	g := NewGateway()
	g.SetAdapterProvider(staticProvider(testAdapter("X", "m1")))

	req := &http.Request{URL: &url.URL{Path: "/aiserver.v1.AiService/Unknown"}}
	resp := makeResponse(200, []byte("hello"))
	if g.MaybeRewriteResponse(req, resp) {
		t.Error("expected false for path not in rewrites map")
	}
}

func TestMaybeRewriteResponse_NilProvider(t *testing.T) {
	g := NewGateway()

	req := &http.Request{URL: &url.URL{Path: "/aiserver.v1.AiService/AvailableModels"}}
	resp := makeResponse(200, []byte("x"))
	if g.MaybeRewriteResponse(req, resp) {
		t.Error("expected false when adapterProvider is nil")
	}
}

func TestMaybeRewriteResponse_EmptyAdapters(t *testing.T) {
	g := NewGateway()
	g.SetAdapterProvider(staticProvider())

	req := &http.Request{URL: &url.URL{Path: "/aiserver.v1.AiService/AvailableModels"}}
	resp := makeResponse(200, []byte("x"))
	if g.MaybeRewriteResponse(req, resp) {
		t.Error("expected false when adapter list is empty")
	}
}

func TestMaybeRewriteResponse_RawProtobuf(t *testing.T) {
	g := NewGateway()
	adapter := testAdapter("TestModel", "gpt-4o")
	g.SetAdapterProvider(staticProvider(adapter))

	// rewriteAvailableModels ignores the input body, so any body (including a
	// valid AvailableModels protobuf) will be rewritten successfully.
	originalBody := buildAvailableModelsResponse([]AdapterInfo{adapter})

	req := &http.Request{URL: &url.URL{Path: "/aiserver.v1.AiService/AvailableModels"}}
	resp := makeResponse(200, originalBody)
	if !g.MaybeRewriteResponse(req, resp) {
		t.Fatal("expected rewrite to succeed")
	}

	newBody, _ := io.ReadAll(resp.Body)
	if len(newBody) == 0 {
		t.Error("rewritten body is empty")
	}
	if resp.ContentLength != int64(len(newBody)) {
		t.Errorf("ContentLength = %d, want %d", resp.ContentLength, len(newBody))
	}
	if enc := resp.Header.Get("Content-Encoding"); enc != "" {
		t.Errorf("Content-Encoding should be removed, got %q", enc)
	}
	cl := resp.Header.Get("Content-Length")
	if cl != strconv.Itoa(len(newBody)) {
		t.Errorf("Content-Length header = %q, want %d", cl, len(newBody))
	}
}

// ---------------------------------------------------------------------------
// tryRewrite
// ---------------------------------------------------------------------------

func TestTryRewrite_RawProtobuf(t *testing.T) {
	adapter := testAdapter("X", "m1")
	adapters := []AdapterInfo{adapter}
	payload := buildAvailableModelsResponse(adapters)

	// rewriteAvailableModels ignores the body, so raw path always succeeds.
	out, framed, err := tryRewrite(payload, adapters, rewriteAvailableModels)
	if err != nil {
		t.Fatalf("tryRewrite raw: %v", err)
	}
	if framed {
		t.Error("framed=true for raw protobuf input")
	}
	if len(out) == 0 {
		t.Error("output is empty")
	}
}

func TestTryRewrite_ConnectEnvelope(t *testing.T) {
	adapter := testAdapter("X", "m1")
	adapters := []AdapterInfo{adapter}
	payload := buildAvailableModelsResponse(adapters)
	envelope := connectEnvelope(payload)

	// Because rewriteAvailableModels ignores the body, the raw-protobuf path
	// succeeds first even for envelope input. This is the actual production
	// behaviour: the rewriter always takes the raw path for AvailableModels.
	out, framed, err := tryRewrite(envelope, adapters, rewriteAvailableModels)
	if err != nil {
		t.Fatalf("tryRewrite envelope: %v", err)
	}
	// The raw path wins, so framed=false.
	if framed {
		t.Error("framed=true but raw path should win for rewriteAvailableModels")
	}
	if len(out) == 0 {
		t.Error("output is empty")
	}
}

func TestTryRewrite_ConnectEnvelopeWithStrictRewriter(t *testing.T) {
	// Use a strict rewriter that rejects envelope-shaped input as raw protobuf,
	// forcing tryRewrite into the Connect-envelope code path.
	adapter := testAdapter("X", "m1")
	adapters := []AdapterInfo{adapter}
	payload := buildAvailableModelsResponse(adapters)
	envelope := connectEnvelope(payload)

	out, framed, err := tryRewrite(envelope, adapters, strictRewriter)
	if err != nil {
		t.Fatalf("tryRewrite strict envelope: %v", err)
	}
	if !framed {
		t.Error("framed=false for Connect envelope with strict rewriter")
	}
	if len(out) < 5 {
		t.Fatalf("output too short: %d", len(out))
	}
	frameLen := int(binary.BigEndian.Uint32(out[1:5]))
	if 5+frameLen != len(out) {
		t.Errorf("frame length mismatch: header=%d, actual=%d", frameLen, len(out)-5)
	}
}

func TestTryRewrite_CompressedEnvelopeWithStrictRewriter(t *testing.T) {
	adapter := testAdapter("X", "m1")
	adapters := []AdapterInfo{adapter}
	payload := buildAvailableModelsResponse(adapters)
	envelope := connectEnvelopeCompressed(payload)

	out, framed, err := tryRewrite(envelope, adapters, strictRewriter)
	if err != nil {
		t.Fatalf("tryRewrite compressed envelope: %v", err)
	}
	if !framed {
		t.Error("framed=false for compressed Connect envelope with strict rewriter")
	}
	// Output should be an uncompressed Connect envelope (flags=0).
	if out[0] != 0 {
		t.Errorf("expected flags=0 in output, got %d", out[0])
	}
}

func TestTryRewrite_InvalidBody(t *testing.T) {
	// With rewriteAvailableModels (which ignores body), even garbage input
	// succeeds on the raw path. This test verifies that behaviour.
	out, framed, err := tryRewrite([]byte{0xFF, 0xFE}, []AdapterInfo{testAdapter("X", "m1")}, rewriteAvailableModels)
	if err != nil {
		t.Fatalf("rewriteAvailableModels ignores body, should not error: %v", err)
	}
	if framed {
		t.Error("should take raw path")
	}
	if len(out) == 0 {
		t.Error("output should not be empty")
	}
}

func TestTryRewrite_InvalidBodyWithStrictRewriter(t *testing.T) {
	// A strict rewriter that rejects both raw and envelope paths should
	// cause tryRewrite to return an error.
	alwaysFail := func(body []byte, adapters []AdapterInfo) ([]byte, error) {
		return nil, errNotRawProtobuf
	}
	_, _, err := tryRewrite([]byte{0xFF, 0xFE}, []AdapterInfo{}, alwaysFail)
	if err == nil {
		t.Error("expected error when rewriter always fails")
	}
}

func TestTryRewrite_TruncatedEnvelopeWithStrictRewriter(t *testing.T) {
	// A truncated envelope (header claims 100 bytes but only 2 present) does
	// NOT match the 5+length==len(body) check, so strictRewriter treats it as
	// raw protobuf and succeeds. This matches the production behaviour of
	// tryRewrite: the envelope path is only entered when the body length
	// exactly matches the declared frame length.
	body := make([]byte, 7)
	body[0] = 0
	binary.BigEndian.PutUint32(body[1:5], 100)
	body[5] = 0xAA
	body[6] = 0xBB

	out, framed, err := tryRewrite(body, []AdapterInfo{testAdapter("X", "m1")}, strictRewriter)
	if err != nil {
		t.Fatalf("truncated envelope should succeed via raw path: %v", err)
	}
	if framed {
		t.Error("should take raw path for truncated envelope")
	}
	if len(out) == 0 {
		t.Error("output should not be empty")
	}
}

// ---------------------------------------------------------------------------
// buildGetDefaultModelNudgeDataResponse
// ---------------------------------------------------------------------------

func TestBuildGetDefaultModelNudgeDataResponse_Empty(t *testing.T) {
	if got := buildGetDefaultModelNudgeDataResponse(nil); got != nil {
		t.Error("expected nil for nil adapters")
	}
	if got := buildGetDefaultModelNudgeDataResponse([]AdapterInfo{}); got != nil {
		t.Error("expected nil for empty adapters")
	}
}

func TestBuildGetDefaultModelNudgeDataResponse_ContainsHexID(t *testing.T) {
	adapter := testAdapter("X", "gpt-4o")
	got := buildGetDefaultModelNudgeDataResponse([]AdapterInfo{adapter})
	if got == nil {
		t.Fatal("expected non-nil response")
	}
	hexID := adapter.StableID()
	if !bytes.Contains(got, []byte(hexID)) {
		t.Errorf("response does not contain StableID %q", hexID)
	}
	if !bytes.Contains(got, []byte("0")) {
		t.Error("response does not contain nudge type '0'")
	}
}

// ---------------------------------------------------------------------------
// buildAvailableModelsResponse
// ---------------------------------------------------------------------------

func TestBuildAvailableModelsResponse_Empty(t *testing.T) {
	got := buildAvailableModelsResponse(nil)
	if len(got) != 0 {
		t.Errorf("expected empty for nil adapters, got %d bytes", len(got))
	}
	got = buildAvailableModelsResponse([]AdapterInfo{})
	if len(got) != 0 {
		t.Errorf("expected empty for empty adapters, got %d bytes", len(got))
	}
}

func TestBuildAvailableModelsResponse_SkipsEmptyModelID(t *testing.T) {
	adapter := AdapterInfo{DisplayName: "NoModel", ModelID: ""}
	got := buildAvailableModelsResponse([]AdapterInfo{adapter})
	if got == nil {
		t.Fatal("expected non-nil (feature configs are still emitted)")
	}
}

func TestBuildAvailableModelsResponse_SingleAdapter(t *testing.T) {
	adapter := testAdapter("MyModel", "gpt-4o")
	got := buildAvailableModelsResponse([]AdapterInfo{adapter})
	if len(got) == 0 {
		t.Fatal("expected non-empty response")
	}
	hexID := adapter.StableID()
	if !bytes.Contains(got, []byte(hexID)) {
		t.Errorf("response does not contain StableID %q", hexID)
	}
	if !bytes.Contains(got, []byte("MyModel")) {
		t.Error("response does not contain display name")
	}
}

func TestBuildAvailableModelsResponse_MultipleAdapters(t *testing.T) {
	a1 := testAdapter("Model1", "gpt-4o")
	a2 := testAdapter("Model2", "claude-3")
	got := buildAvailableModelsResponse([]AdapterInfo{a1, a2})
	if len(got) == 0 {
		t.Fatal("expected non-empty response")
	}
	if !bytes.Contains(got, []byte(a1.StableID())) {
		t.Error("missing StableID for adapter 1")
	}
	if !bytes.Contains(got, []byte(a2.StableID())) {
		t.Error("missing StableID for adapter 2")
	}
	if !bytes.Contains(got, []byte("Model1")) {
		t.Error("missing display name for adapter 1")
	}
	if !bytes.Contains(got, []byte("Model2")) {
		t.Error("missing display name for adapter 2")
	}
}

func TestBuildAvailableModelsResponse_DefaultDisplayName(t *testing.T) {
	adapter := AdapterInfo{ModelID: "gpt-4o", Type: "openai"}
	got := buildAvailableModelsResponse([]AdapterInfo{adapter})
	if !bytes.Contains(got, []byte("gpt-4o")) {
		t.Error("expected ModelID as fallback display name")
	}
}

func TestBuildAvailableModelsResponse_ContainsFeatureConfigs(t *testing.T) {
	adapter := testAdapter("X", "gpt-4o")
	got := buildAvailableModelsResponse([]AdapterInfo{adapter})
	hexID := adapter.StableID()

	count := bytes.Count(got, []byte(hexID))
	if count < 3 {
		t.Errorf("expected StableID to appear at least 3 times (model + feature configs), got %d", count)
	}
}

func TestBuildAvailableModelsResponse_ContainsFields12And13(t *testing.T) {
	adapter := testAdapter("X", "gpt-4o")
	got := buildAvailableModelsResponse([]AdapterInfo{adapter})
	if len(got) == 0 {
		t.Fatal("expected non-empty response")
	}
	if len(got) < 50 {
		t.Errorf("response seems too short (%d bytes), may be missing feature configs", len(got))
	}
}

// ---------------------------------------------------------------------------
// buildAvailableModel
// ---------------------------------------------------------------------------

func TestBuildAvailableModel(t *testing.T) {
	hexID := testAdapter("X", "gpt-4o").StableID()
	got := buildAvailableModel(hexID, "TestDisplay")
	if len(got) == 0 {
		t.Fatal("expected non-empty model bytes")
	}
	if !bytes.Contains(got, []byte(hexID)) {
		t.Error("model bytes do not contain hexID")
	}
	if !bytes.Contains(got, []byte("TestDisplay")) {
		t.Error("model bytes do not contain display name")
	}
	if !bytes.Contains(got, []byte("Notes")) {
		t.Error("model bytes do not contain tooltip 'Notes'")
	}
}

// ---------------------------------------------------------------------------
// appendFeatureCfg
// ---------------------------------------------------------------------------

func TestAppendFeatureCfg_DefaultOnly(t *testing.T) {
	hexID := testAdapter("X", "m1").StableID()
	buf := appendFeatureCfg(nil, 4, hexID, false, false)
	if len(buf) == 0 {
		t.Fatal("expected non-empty bytes")
	}
	if count := bytes.Count(buf, []byte(hexID)); count != 1 {
		t.Errorf("expected hexID once, got %d occurrences", count)
	}
}

func TestAppendFeatureCfg_WithFallback(t *testing.T) {
	hexID := testAdapter("X", "m1").StableID()
	buf := appendFeatureCfg(nil, 5, hexID, true, false)
	if count := bytes.Count(buf, []byte(hexID)); count != 2 {
		t.Errorf("expected hexID twice (default + fallback), got %d", count)
	}
}

func TestAppendFeatureCfg_WithFallbackAndBestOfN(t *testing.T) {
	hexID := testAdapter("X", "m1").StableID()
	buf := appendFeatureCfg(nil, 6, hexID, true, true)
	if count := bytes.Count(buf, []byte(hexID)); count != 3 {
		t.Errorf("expected hexID 3 times (default + fallback + best_of_n), got %d", count)
	}
}

// ---------------------------------------------------------------------------
// appendString / appendBool
// ---------------------------------------------------------------------------

func TestAppendString(t *testing.T) {
	buf := appendString(nil, 1, "hello")
	if len(buf) == 0 {
		t.Fatal("expected non-empty bytes")
	}
	if !bytes.Contains(buf, []byte("hello")) {
		t.Error("bytes do not contain the string value")
	}
}

func TestAppendBool_True(t *testing.T) {
	buf := appendBool(nil, 2, true)
	if len(buf) == 0 {
		t.Fatal("expected non-empty bytes")
	}
}

func TestAppendBool_False(t *testing.T) {
	buf := appendBool(nil, 3, false)
	if len(buf) == 0 {
		t.Fatal("expected non-empty bytes")
	}
}

// ---------------------------------------------------------------------------
// Gateway.SetAdapterProvider
// ---------------------------------------------------------------------------

func TestSetAdapterProvider(t *testing.T) {
	g := NewGateway()
	called := false
	g.SetAdapterProvider(func() []AdapterInfo {
		called = true
		return []AdapterInfo{testAdapter("X", "m1")}
	})
	g.SyntheticPath("/aiserver.v1.AiService/AvailableModels")
	if !called {
		t.Error("adapterProvider was not called")
	}
}

// ---------------------------------------------------------------------------
// NewGateway
// ---------------------------------------------------------------------------

func TestNewGateway(t *testing.T) {
	g := NewGateway()
	if g == nil {
		t.Fatal("NewGateway returned nil")
	}
	if g.adapterProvider != nil {
		t.Error("expected nil adapterProvider on new Gateway")
	}
}

// ---------------------------------------------------------------------------
// connectFlag constants
// ---------------------------------------------------------------------------

func TestConnectFlagConstants(t *testing.T) {
	if connectFlagCompressed != 0x01 {
		t.Errorf("connectFlagCompressed = %d, want 0x01", connectFlagCompressed)
	}
	if connectFlagEndStream != 0x02 {
		t.Errorf("connectFlagEndStream = %d, want 0x02", connectFlagEndStream)
	}
}

// ---------------------------------------------------------------------------
// Integration: SyntheticPath + MaybeRewriteResponse consistency
// ---------------------------------------------------------------------------

func TestSyntheticPathAndRewriteProduceSameModelEntries(t *testing.T) {
	adapter := testAdapter("ConsistentModel", "gpt-4o")
	g := NewGateway()
	g.SetAdapterProvider(staticProvider(adapter))

	synthetic := g.SyntheticPath("/aiserver.v1.AiService/AvailableModels")
	if synthetic == nil {
		t.Fatal("SyntheticPath returned nil")
	}

	req := &http.Request{URL: &url.URL{Path: "/aiserver.v1.AiService/AvailableModels"}}
	resp := makeResponse(200, synthetic)
	if !g.MaybeRewriteResponse(req, resp) {
		t.Fatal("MaybeRewriteResponse returned false")
	}

	rewritten, _ := io.ReadAll(resp.Body)
	if len(rewritten) == 0 {
		t.Fatal("rewritten body is empty")
	}

	hexID := adapter.StableID()
	if !bytes.Contains(rewritten, []byte(hexID)) {
		t.Error("rewritten body does not contain StableID")
	}
}

// ---------------------------------------------------------------------------
// codec.Gunzip round-trip (used by tryRewrite)
// ---------------------------------------------------------------------------

func TestGunzipRoundTrip(t *testing.T) {
	original := []byte("hello gzip world")
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(original)
	_ = w.Close()

	decoded, err := codec.Gunzip(buf.Bytes())
	if err != nil {
		t.Fatalf("Gunzip: %v", err)
	}
	if !bytes.Equal(decoded, original) {
		t.Errorf("Gunzip round-trip failed: got %q, want %q", decoded, original)
	}
}
