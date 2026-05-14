package mitm

import (
	"bytes"
	"io"
	"net/http"

	"cursorbridge/internal/agent"
)

// handleBidiAppend reads the full request body (Cursor sends BidiAppend as a
// unary POST), hands it to the agent package, and packages the result as a
// goproxy-compatible http.Response.
func handleBidiAppend(req *http.Request) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleBidiAppend(body, req.Header.Get("Content-Type"))
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{
		Status:        http.StatusText(res.Status),
		StatusCode:    res.Status,
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Body:          io.NopCloser(bytes.NewReader(res.Body)),
		ContentLength: int64(len(res.Body)),
		Header:        hdr,
		Request:       req,
	}
}

// handleRunSSE returns a streaming http.Response whose Body is an io.Pipe.
func handleRunSSE(req *http.Request, resolver agent.AdapterResolver) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		agent.HandleRunSSE(req.Context(), body, req.Header.Get("Content-Type"), pw, resolver, agent.DefaultDeps)
	}()
	hdr := http.Header{}
	for k, vs := range agent.RunSSEHeaders {
		for _, v := range vs {
			hdr.Add(k, v)
		}
	}
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      req.Proto,
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
		Body:       pr,
		Header:     hdr,
		Request:    req,
	}
}

func handleWriteGitCommitMessage(req *http.Request, resolver agent.AdapterResolver, selectedModel string) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleWriteGitCommitMessage(req.Context(), body, req.Header.Get("Content-Type"), resolver, selectedModel)
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{
		Status:        http.StatusText(res.Status),
		StatusCode:    res.Status,
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Body:          io.NopCloser(bytes.NewReader(res.Body)),
		ContentLength: int64(len(res.Body)),
		Header:        hdr,
		Request:       req,
	}
}

func handleBugBotRunSSE(req *http.Request, resolver agent.AdapterResolver, selectedModel string) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		agent.HandleBugBotRunSSE(req.Context(), body, req.Header.Get("Content-Type"), pw, resolver, selectedModel)
	}()
	hdr := http.Header{}
	for k, vs := range agent.BugBotSSEHeaders {
		for _, v := range vs {
			hdr.Add(k, v)
		}
	}
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      req.Proto,
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
		Body:       pr,
		Header:     hdr,
		Request:    req,
	}
}

func handleBackgroundComposerAttach(req *http.Request, resolver agent.AdapterResolver) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		agent.HandleBackgroundComposerAttach(req.Context(), body, req.Header.Get("Content-Type"), pw, resolver)
	}()
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      req.Proto,
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
		Body:       pr,
		Header: http.Header{
			"Content-Type":             {"text/event-stream"},
			"Cache-Control":            {"no-cache"},
			"Connect-Content-Encoding": {"gzip"},
			"Connect-Accept-Encoding":  {"gzip"},
		},
		Request: req,
	}
}

func handleBackgroundComposerInteractionUpdates(req *http.Request) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		agent.HandleBackgroundComposerInteractionUpdates(req.Context(), body, req.Header.Get("Content-Type"), pw)
	}()
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      req.Proto,
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
		Body:       pr,
		Header: http.Header{
			"Content-Type":             {"text/event-stream"},
			"Cache-Control":            {"no-cache"},
			"Connect-Content-Encoding": {"gzip"},
			"Connect-Accept-Encoding":  {"gzip"},
		},
		Request: req,
	}
}

func handleBackgroundComposerAddFollowup(req *http.Request) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleAddAsyncBackgroundComposer(body, req.Header.Get("Content-Type"))
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{Status: http.StatusText(res.Status), StatusCode: res.Status, Proto: req.Proto, ProtoMajor: req.ProtoMajor, ProtoMinor: req.ProtoMinor, Body: io.NopCloser(bytes.NewReader(res.Body)), ContentLength: int64(len(res.Body)), Header: hdr, Request: req}
}

func handleBackgroundComposerStatus(req *http.Request) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleGetBackgroundComposerStatus(body, req.Header.Get("Content-Type"))
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{Status: http.StatusText(res.Status), StatusCode: res.Status, Proto: req.Proto, ProtoMajor: req.ProtoMajor, ProtoMinor: req.ProtoMinor, Body: io.NopCloser(bytes.NewReader(res.Body)), ContentLength: int64(len(res.Body)), Header: hdr, Request: req}
}

func handleGetTerminalCompletion(req *http.Request, resolver agent.AdapterResolver, selectedModel string) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleGetTerminalCompletion(req.Context(), body, req.Header.Get("Content-Type"), resolver, selectedModel)
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{Status: http.StatusText(res.Status), StatusCode: res.Status, Proto: req.Proto, ProtoMajor: req.ProtoMajor, ProtoMinor: req.ProtoMinor, Body: io.NopCloser(bytes.NewReader(res.Body)), ContentLength: int64(len(res.Body)), Header: hdr, Request: req}
}

func handleGetChatSuggestions(req *http.Request, resolver agent.AdapterResolver, selectedModel string) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleGetChatSuggestions(req.Context(), body, req.Header.Get("Content-Type"), resolver, selectedModel)
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{Status: http.StatusText(res.Status), StatusCode: res.Status, Proto: req.Proto, ProtoMajor: req.ProtoMajor, ProtoMinor: req.ProtoMinor, Body: io.NopCloser(bytes.NewReader(res.Body)), ContentLength: int64(len(res.Body)), Header: hdr, Request: req}
}

func handleGetConversationSummary(req *http.Request) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleGetConversationSummary(body, req.Header.Get("Content-Type"))
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{Status: http.StatusText(res.Status), StatusCode: res.Status, Proto: req.Proto, ProtoMajor: req.ProtoMajor, ProtoMinor: req.ProtoMinor, Body: io.NopCloser(bytes.NewReader(res.Body)), ContentLength: int64(len(res.Body)), Header: hdr, Request: req}
}

func handleGenerateTldr(req *http.Request, resolver agent.AdapterResolver, selectedModel string) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleGenerateTldr(req.Context(), body, req.Header.Get("Content-Type"), resolver, selectedModel)
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{Status: http.StatusText(res.Status), StatusCode: res.Status, Proto: req.Proto, ProtoMajor: req.ProtoMajor, ProtoMinor: req.ProtoMinor, Body: io.NopCloser(bytes.NewReader(res.Body)), ContentLength: int64(len(res.Body)), Header: hdr, Request: req}
}

func handleLintExplanation(req *http.Request, resolver agent.AdapterResolver, selectedModel string) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleLintExplanation(req.Context(), body, req.Header.Get("Content-Type"), resolver, selectedModel)
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{Status: http.StatusText(res.Status), StatusCode: res.Status, Proto: req.Proto, ProtoMajor: req.ProtoMajor, ProtoMinor: req.ProtoMinor, Body: io.NopCloser(bytes.NewReader(res.Body)), ContentLength: int64(len(res.Body)), Header: hdr, Request: req}
}

func handleStreamDiffReview(req *http.Request, resolver agent.AdapterResolver, selectedModel string) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		agent.HandleStreamDiffReview(req.Context(), body, req.Header.Get("Content-Type"), pw, resolver, selectedModel)
	}()
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      req.Proto,
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
		Body:       pr,
		Header: http.Header{
			"Content-Type":             {"text/event-stream"},
			"Cache-Control":            {"no-cache"},
			"Connect-Content-Encoding": {"gzip"},
			"Connect-Accept-Encoding":  {"gzip"},
		},
		Request: req,
	}
}

func handleCountTokens(req *http.Request) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleCountTokens(body, req.Header.Get("Content-Type"))
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{Status: http.StatusText(res.Status), StatusCode: res.Status, Proto: req.Proto, ProtoMajor: req.ProtoMajor, ProtoMinor: req.ProtoMinor, Body: io.NopCloser(bytes.NewReader(res.Body)), ContentLength: int64(len(res.Body)), Header: hdr, Request: req}
}

func handleGetMcpTools(req *http.Request) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleGetMcpTools(body, req.Header.Get("Content-Type"))
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{Status: http.StatusText(res.Status), StatusCode: res.Status, Proto: req.Proto, ProtoMajor: req.ProtoMajor, ProtoMinor: req.ProtoMinor, Body: io.NopCloser(bytes.NewReader(res.Body)), ContentLength: int64(len(res.Body)), Header: hdr, Request: req}
}

func handleGetChatTitle(req *http.Request, resolver agent.AdapterResolver, selectedModel string) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"code":"invalid_argument","message":"read body: `+err.Error()+`"}`)
	}
	res := agent.HandleGetChatTitle(req.Context(), body, req.Header.Get("Content-Type"), resolver, selectedModel)
	hdr := http.Header{}
	if res.ContentType != "" {
		hdr.Set("Content-Type", res.ContentType)
	}
	return &http.Response{Status: http.StatusText(res.Status), StatusCode: res.Status, Proto: req.Proto, ProtoMajor: req.ProtoMajor, ProtoMinor: req.ProtoMinor, Body: io.NopCloser(bytes.NewReader(res.Body)), ContentLength: int64(len(res.Body)), Header: hdr, Request: req}
}
