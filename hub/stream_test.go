package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
)

func TestStreamingChatCompletions(t *testing.T) {
	handler := &stubHandler{
		streamBody: [][]byte{
			[]byte("data: {\"chunk\":1}\n\n"),
			[]byte("data: {\"chunk\":2}\n\n"),
		},
	}
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{})
	reg.SetWorkReadyForTest(true)
	h := newTestHubHTTP(reg, handler, HTTPConfig{})

	body := []byte(`{"model":"llama3","stream":true,"messages":[]}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q", ct)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "data: {\"chunk\":1}") {
		t.Fatalf("missing chunk1: %q", out)
	}
	if !strings.Contains(out, "data: [DONE]") {
		t.Fatalf("missing DONE: %q", out)
	}
	if handler.streamReq == nil {
		t.Fatal("expected stream dispatch")
	}
}

func TestDispatchStreamRoundTrip(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{})

	stream := &callbackWorkStream{
		onSend: func(req *pb.ChatRequest) {
			reg.FulfillChatResponse(&pb.ChatResponse{
				RequestId:      req.GetRequestId(),
				OpenaiJsonBody: []byte("data: {}\n\n"),
			})
			reg.FulfillChatResponse(&pb.ChatResponse{
				RequestId: req.GetRequestId(),
				StreamEnd: true,
			})
		},
	}
	reg.SetWorkStream(stream)

	ch, err := reg.DispatchStreamChat(context.Background(), &pb.ChatRequest{
		RequestId:      "stream-1",
		OpenaiJsonBody: []byte(`{"stream":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var chunks int
	for range ch {
		chunks++
	}
	if chunks != 2 {
		t.Fatalf("chunks = %d", chunks)
	}
}
