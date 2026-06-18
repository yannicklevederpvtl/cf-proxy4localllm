package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
)

func TestChatErrorHTTPStatus(t *testing.T) {
	if got := chatErrorHTTPStatus(context.DeadlineExceeded); got != http.StatusGatewayTimeout {
		t.Fatalf("deadline = %d", got)
	}
	if got := chatErrorHTTPStatus(errors.New("upstream failed")); got != http.StatusBadGateway {
		t.Fatalf("generic = %d", got)
	}
}

func TestHealthEnhancedFields(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{
		OllamaBaseURL: "http://127.0.0.1:11434/v1",
		DefaultModel:  "llama3.2",
	})
	reg.SetWorkReadyForTest(true)
	h := newTestHubHTTP(reg, &stubHandler{}, HTTPConfig{ModelAlias: "local-ollama"})
	h.metrics.RecordRequest(false)
	h.metrics.RecordRequest(true)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{
		"bridge_ollama_url",
		"bridge_default_model",
		"uptime_seconds",
		"requests_total",
		"requests_failed",
	} {
		if body[key] == nil {
			t.Fatalf("missing %s in %+v", key, body)
		}
	}
	if body["bridge_ollama_url"] != "http://127.0.0.1:11434/v1" {
		t.Fatalf("bridge_ollama_url = %v", body["bridge_ollama_url"])
	}
	if body["bridge_default_model"] != "llama3.2" {
		t.Fatalf("bridge_default_model = %v", body["bridge_default_model"])
	}
	if body["requests_total"].(float64) != 2 {
		t.Fatalf("requests_total = %v", body["requests_total"])
	}
	if body["requests_failed"].(float64) != 1 {
		t.Fatalf("requests_failed = %v", body["requests_failed"])
	}
}

type slowHandler struct {
	delay time.Duration
}

func (s *slowHandler) CompleteChat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	select {
	case <-time.After(s.delay):
		return &pb.ChatResponse{OpenaiJsonBody: req.GetOpenaiJsonBody()}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *slowHandler) DispatchStream(_ context.Context, _ *pb.ChatRequest) (<-chan *pb.ChatResponse, error) {
	return nil, context.DeadlineExceeded
}

func TestChatCompletionsTimeoutReturns504(t *testing.T) {
	orig := upstreamRequestTimeout
	upstreamRequestTimeout = 50 * time.Millisecond
	t.Cleanup(func() { upstreamRequestTimeout = orig })

	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{})
	reg.SetWorkReadyForTest(true)
	h := newTestHubHTTP(reg, &slowHandler{delay: 2 * time.Second}, HTTPConfig{})

	body := []byte(`{"model":"llama3","messages":[]}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)))

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if h.metrics.Failed() != 1 {
		t.Fatalf("failed count = %d", h.metrics.Failed())
	}
}

func TestLoggingMiddlewareSkipsHealth(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	loggingMiddleware(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if !called {
		t.Fatal("expected handler to run")
	}
}

func TestLoggingMiddlewarePreservesFlusher(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected Flusher on wrapped writer")
		}
		f.Flush()
	})
	base := &flushRecorder{}
	loggingMiddleware(next).ServeHTTP(base, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	if !base.flushed {
		t.Fatal("expected Flush to reach underlying writer")
	}
}

type flushRecorder struct {
	httptest.ResponseRecorder
	flushed bool
}

func (f *flushRecorder) Flush() {
	f.flushed = true
}
