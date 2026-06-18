package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
)

func TestForwardToOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cfg := Config{OllamaBaseURL: srv.URL + "/v1"}
	payload := []byte(`{"model":"llama3","messages":[{"role":"user","content":"hi"}]}`)
	resp, err := forwardToOllama(context.Background(), cfg, payload)
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]any
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload, &want); err != nil {
		t.Fatal(err)
	}
	if got["model"] != want["model"] {
		t.Fatalf("model = %v, want %v", got["model"], want["model"])
	}
}

func TestResolveModelForUpstreamOllama(t *testing.T) {
	cfg := Config{
		ModelAlias:      "local-ollama",
		DefaultModel:    "qwen3:8b",
		UpstreamBaseURL: "http://127.0.0.1:11434/v1",
	}
	out, err := resolveModelForUpstream([]byte(`{"model":"local-ollama","messages":[]}`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "qwen3:8b" {
		t.Fatalf("model = %v", payload["model"])
	}
	if payload["think"] != false {
		t.Fatalf("think = %v", payload["think"])
	}
}

func TestResolveModelForUpstreamOpenAI(t *testing.T) {
	cfg := Config{
		ModelAlias:      "local-ollama",
		DefaultModel:    "gpt-4.1-mini",
		UpstreamBaseURL: "https://api.openai.com/v1",
	}
	out, err := resolveModelForUpstream([]byte(`{"model":"local-ollama","messages":[]}`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "gpt-4.1-mini" {
		t.Fatalf("model = %v", payload["model"])
	}
	if _, ok := payload["think"]; ok {
		t.Fatal("think should not be set for OpenAI upstream")
	}
}

func TestForwardUpstreamSendsAPIKey(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gpt-4.1-mini",
			"choices": []map[string]any{
				{"message": map[string]string{"content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	cfg := Config{
		UpstreamBaseURL: srv.URL + "/v1",
		UpstreamAPIKey:  "sk-test-key",
		ModelAlias:      "local-ollama",
		DefaultModel:    "gpt-4.1-mini",
	}
	_, err := forwardToOllama(context.Background(), cfg, []byte(`{"model":"local-ollama","messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer sk-test-key" {
		t.Fatalf("auth = %q", auth)
	}
}

func TestSanitizeOpenAIJSON(t *testing.T) {
	raw := []byte(`{"model":"qwen3:8b","choices":[{"delta":{"content":"hi","reasoning":"secret"}}]}`)
	out := sanitizeOpenAIJSON(raw, "local-ollama")
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "local-ollama" {
		t.Fatalf("model = %v", payload["model"])
	}
	choices := payload["choices"].([]any)
	delta := choices[0].(map[string]any)["delta"].(map[string]any)
	if _, ok := delta["reasoning"]; ok {
		t.Fatal("reasoning should be stripped")
	}
}

func TestSanitizeSSELine(t *testing.T) {
	line := []byte(`data: {"model":"qwen3:8b","choices":[{"delta":{"content":"hi","reasoning":"x"}}]}`)
	out := sanitizeSSELine(line, "local-ollama")
	if !bytes.Contains(out, []byte(`"model":"local-ollama"`)) {
		t.Fatalf("unexpected: %s", out)
	}
	if bytes.Contains(out, []byte("reasoning")) {
		t.Fatalf("reasoning not stripped: %s", out)
	}
}

func TestIsStreamRequest(t *testing.T) {
	if !isStreamRequest([]byte(`{"stream":true}`)) {
		t.Fatal("expected stream true")
	}
	if isStreamRequest([]byte(`{"stream":false}`)) {
		t.Fatal("expected stream false")
	}
}

func TestEnsureNonStreamingBody(t *testing.T) {
	out, err := ensureNonStreamingBody([]byte(`{"model":"llama3","stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if isStreamRequest(out) {
		t.Fatalf("stream still true: %s", out)
	}
}

func TestForwardToOllamaStripsStreamWhenForced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if isStreamRequest(body) {
			http.Error(w, "expected non-stream", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "chat.completion",
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "hello"}},
			},
		})
	}))
	defer srv.Close()

	cfg := Config{OllamaBaseURL: srv.URL + "/v1"}
	resp, err := handleChatRequest(context.Background(), cfg, &pb.ChatRequest{
		RequestId:      "req-1",
		OpenaiJsonBody: []byte(`{"model":"llama3","stream":false,"messages":[]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isOpenAIChatCompletion(resp.GetOpenaiJsonBody()) {
		t.Fatalf("not openai shape: %s", resp.GetOpenaiJsonBody())
	}
}

func TestHandleChatRequestUsesOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "hello"}},
			},
		})
	}))
	defer srv.Close()

	cfg := Config{OllamaBaseURL: srv.URL + "/v1"}
	resp, err := handleChatRequest(context.Background(), cfg, &pb.ChatRequest{
		RequestId:      "req-1",
		OpenaiJsonBody: []byte(`{"model":"llama3","messages":[]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetRequestId() != "req-1" {
		t.Fatalf("request_id = %q", resp.GetRequestId())
	}
	var body map[string]any
	if err := json.Unmarshal(resp.GetOpenaiJsonBody(), &body); err != nil {
		t.Fatal(err)
	}
}
