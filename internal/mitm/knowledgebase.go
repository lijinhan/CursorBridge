package mitm

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"cursorbridge/internal/knowledgebase"
	"cursorbridge/internal/logutil"
	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"

	"google.golang.org/protobuf/proto"
)

// handleKnowledgeBase processes KnowledgeBaseAdd/List/Get/Remove requests
// using local storage instead of upstream.
func handleKnowledgeBase(req *http.Request, path string) *http.Response {
	body, err := readDecodedBody(req)
	if err != nil {
		return makeJSONResp(req, http.StatusBadRequest, `{"error":"read body"}`)
	}

	var respBody []byte

	switch {
	case strings.HasSuffix(path, "KnowledgeBaseAdd"):
		respBody = handleKBAdd(body, req.Header.Get("Content-Type"))
	case strings.HasSuffix(path, "KnowledgeBaseList"):
		respBody = handleKBList(body, req.Header.Get("Content-Type"))
	case strings.HasSuffix(path, "KnowledgeBaseGet"):
		respBody = handleKBGet(body, req.Header.Get("Content-Type"))
	case strings.HasSuffix(path, "KnowledgeBaseRemove"):
		respBody = handleKBRemove(body, req.Header.Get("Content-Type"))
	default:
		respBody = nil
	}

	return &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Body:          io.NopCloser(bytes.NewReader(respBody)),
		ContentLength: int64(len(respBody)),
		Header: http.Header{
			"Content-Type": {"application/proto"},
		},
		Request: req,
	}
}

func handleKBAdd(body []byte, contentType string) []byte {
	addReq := &aiserverv1.KnowledgeBaseAddRequest{}
	if err := decodeUnaryMsg(body, contentType, addReq); err != nil {
		logutil.Warn("KB decode add request", "error", err)
		return nil
	}
	id := knowledgebase.Add(addReq.GetKnowledge(), addReq.GetTitle(), addReq.GetGitOrigin())
	resp := &aiserverv1.KnowledgeBaseAddResponse{
		Success: true,
		Id:      id,
	}
	out, _ := proto.Marshal(resp)
	return out
}

func handleKBList(body []byte, contentType string) []byte {
	listReq := &aiserverv1.KnowledgeBaseListRequest{}
	_ = decodeUnaryMsg(body, contentType, listReq)
	items := knowledgebase.List(listReq.GetGitOrigin())
	resp := &aiserverv1.KnowledgeBaseListResponse{
		Success: true,
	}
	for _, it := range items {
		resp.AllResults = append(resp.AllResults, &aiserverv1.KnowledgeBaseListResponse_Item{
			Id:          it.ID,
			Knowledge:   it.Knowledge,
			Title:       it.Title,
			CreatedAt:   it.CreatedAt,
			IsGenerated: it.IsGenerated,
		})
	}
	out, _ := proto.Marshal(resp)
	return out
}

func handleKBGet(body []byte, contentType string) []byte {
	getReq := &aiserverv1.KnowledgeBaseGetRequest{}
	if err := decodeUnaryMsg(body, contentType, getReq); err != nil {
		return nil
	}
	item, ok := knowledgebase.Get(getReq.GetId())
	resp := &aiserverv1.KnowledgeBaseGetResponse{
		Success: ok,
	}
	if ok {
		resp.Result = &aiserverv1.KnowledgeBaseGetResponse_Item{
			Id:        item.ID,
			Knowledge: item.Knowledge,
			Title:     item.Title,
			CreatedAt: item.CreatedAt,
		}
	}
	out, _ := proto.Marshal(resp)
	return out
}

func handleKBRemove(body []byte, contentType string) []byte {
	rmReq := &aiserverv1.KnowledgeBaseRemoveRequest{}
	if err := decodeUnaryMsg(body, contentType, rmReq); err != nil {
		return nil
	}
	ok := knowledgebase.Remove(rmReq.GetId())
	resp := &aiserverv1.KnowledgeBaseRemoveResponse{
		Success: ok,
	}
	out, _ := proto.Marshal(resp)
	return out
}

// decodeUnaryMsg strips the optional Connect envelope before unmarshalling proto.
func decodeUnaryMsg(body []byte, contentType string, msg proto.Message) error {
	payload := body
	if strings.Contains(strings.ToLower(contentType), "connect") && len(payload) >= 5 {
		flags := payload[0]
		length := int(payload[1])<<24 | int(payload[2])<<16 | int(payload[3])<<8 | int(payload[4])
		if length >= 0 && len(payload) >= 5+length {
			payload = payload[5 : 5+length]
			if flags&0x01 != 0 {
				r, gerr := gzip.NewReader(bytes.NewReader(payload))
				if gerr != nil {
					return gerr
				}
				defer r.Close()
				out, rerr := io.ReadAll(r)
				if rerr != nil {
					return rerr
				}
				payload = out
			}
		}
	}
	return proto.Unmarshal(payload, msg)
}
