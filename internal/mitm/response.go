package mitm

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

func gzipReader(b []byte) (io.ReadCloser, error) {
	return gzip.NewReader(bytes.NewReader(b))
}

// mockProto returns an empty 200 OK response with application/proto content type.
func mockProto(req *http.Request, body []byte) *http.Response {
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Header: http.Header{
			"Content-Type": {"application/proto"},
		},
		Request: req,
	}
}

// mock404 returns a 404 Not Found text response.
func mock404(req *http.Request) *http.Response {
	body := "404 未找到\n"
	return &http.Response{
		Status:        "404 Not Found",
		StatusCode:    http.StatusNotFound,
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Header: http.Header{
			"Content-Type":           {"text/plain; charset=utf-8"},
			"X-Content-Type-Options": {"nosniff"},
		},
		Request: req,
	}
}

// makeJSONResp returns a JSON response with the given status and body.
func makeJSONResp(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Header:        http.Header{"Content-Type": {"application/json"}},
		Request:       req,
	}
}

// readDecodedBody reads req.Body and transparently un-gzips when Cursor
// shipped the payload with HTTP-level gzip (Content-Encoding: gzip).
func readDecodedBody(req *http.Request) ([]byte, error) {
	defer req.Body.Close()
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	enc := strings.ToLower(strings.TrimSpace(req.Header.Get("Content-Encoding")))
	if enc == "gzip" || enc == "x-gzip" {
		zr, gerr := gzipReader(raw)
		if gerr != nil {
			return nil, gerr
		}
		defer zr.Close()
		out, rerr := io.ReadAll(zr)
		if rerr != nil {
			return nil, rerr
		}
		return out, nil
	}
	return raw, nil
}
