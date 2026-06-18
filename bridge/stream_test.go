package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
)

func TestStreamOllamaSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"a\":1}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	var lines []string
	count, err := streamOllamaSSE(context.Background(), Config{OllamaBaseURL: srv.URL + "/v1"}, []byte(`{"stream":true}`), func(chunk []byte, done bool) error {
		if done {
			return nil
		}
		lines = append(lines, string(chunk))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count < 1 {
		t.Fatalf("count = %d", count)
	}
	if !strings.Contains(strings.Join(lines, ""), "data:") {
		t.Fatalf("lines = %v", lines)
	}
}

func TestHandleStreamChatRequestSendsChunks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "data: {\"x\":1}\n\n")
	}))
	defer srv.Close()

	var chunks int
	var sawDone bool
	err := handleStreamChatRequest(context.Background(), Config{OllamaBaseURL: srv.URL + "/v1"}, &pb.ChatRequest{
		RequestId:      "s1",
		OpenaiJsonBody: []byte(`{"stream":true,"model":"llama3"}`),
	}, func(chunk []byte, done bool) error {
		if done {
			sawDone = true
			return nil
		}
		chunks++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if chunks < 1 || !sawDone {
		t.Fatalf("chunks=%d sawDone=%v", chunks, sawDone)
	}
}
