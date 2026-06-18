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

type stubHandler struct {
	lastReq    *pb.ChatRequest
	streamReq  *pb.ChatRequest
	streamBody [][]byte
}

func (s *stubHandler) CompleteChat(_ context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	s.lastReq = req
	return &pb.ChatResponse{OpenaiJsonBody: req.GetOpenaiJsonBody()}, nil
}

func (s *stubHandler) DispatchStream(_ context.Context, req *pb.ChatRequest) (<-chan *pb.ChatResponse, error) {
	s.streamReq = req
	ch := make(chan *pb.ChatResponse, 4)
	for _, line := range s.streamBody {
		ch <- &pb.ChatResponse{RequestId: req.GetRequestId(), OpenaiJsonBody: line}
	}
	ch <- &pb.ChatResponse{RequestId: req.GetRequestId(), StreamEnd: true}
	close(ch)
	return ch, nil
}

func newTestHubHTTP(reg *BridgeRegistry, handler chatHandler, cfg HTTPConfig) *HubHTTP {
	return newHubHTTP(reg, handler, cfg)
}

func TestHealthNoBridge(t *testing.T) {
	h := newTestHubHTTP(NewBridgeRegistry(), &stubHandler{}, HTTPConfig{ModelAlias: "local-ollama"})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %v", body["status"])
	}
	if body["bridge_connected"] != false {
		t.Fatalf("bridge_connected = %v", body["bridge_connected"])
	}
}

func TestHealthWithBridge(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{DefaultModel: "llama3"})
	reg.SetWorkReadyForTest(true)
	h := newTestHubHTTP(reg, &stubHandler{}, HTTPConfig{ModelAlias: "local-ollama"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["bridge_connected"] != true {
		t.Fatalf("bridge_connected = %v", body["bridge_connected"])
	}
	if body["bridge_registered_at"] == nil || body["bridge_last_seen"] == nil {
		t.Fatalf("missing timestamps: %+v", body)
	}
}

func TestHealthRegisteredWithoutWorkStream(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{DefaultModel: "llama3"})
	h := newTestHubHTTP(reg, &stubHandler{}, HTTPConfig{})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["bridge_connected"] != false {
		t.Fatalf("bridge_connected = %v", body["bridge_connected"])
	}
	if body["bridge_work_ready"] != false {
		t.Fatalf("bridge_work_ready = %v", body["bridge_work_ready"])
	}
	if body["bridge_registered_at"] == nil {
		t.Fatal("expected registered_at when bridge metadata present")
	}
}

func TestModelsList(t *testing.T) {
	h := newTestHubHTTP(NewBridgeRegistry(), &stubHandler{}, HTTPConfig{ModelAlias: "my-model"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Object != "list" || len(body.Data) != 1 || body.Data[0].ID != "my-model" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestChatCompletionsNoBridge(t *testing.T) {
	h := newTestHubHTTP(NewBridgeRegistry(), &stubHandler{}, HTTPConfig{})
	body := []byte(`{"model":"llama3","messages":[]}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error.Type != "bridge_unavailable" || resp.Error.Code != 503 {
		t.Fatalf("error = %+v", resp.Error)
	}
}

func TestChatCompletionsEcho(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{DefaultModel: "llama3"})
	reg.SetWorkReadyForTest(true)
	handler := &stubHandler{}
	h := newTestHubHTTP(reg, handler, HTTPConfig{})

	body := []byte(`{"model":"llama3","messages":[{"role":"user","content":"hi"}]}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != string(body) {
		t.Fatalf("echo mismatch: %q", got)
	}
	if handler.lastReq == nil || handler.lastReq.GetRequestId() == "" {
		t.Fatal("expected request id on CompleteChat call")
	}
}

func TestChatCompletionsRequiresAuthWhenEnabled(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{})
	reg.SetWorkReadyForTest(true)
	h := newTestHubHTTP(reg, &stubHandler{}, HTTPConfig{
		ValidateInbound: true,
		InboundAPIKeys:  map[string]struct{}{"secret-key": {}},
	})

	body := []byte(`{"model":"llama3","messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth: status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret-key")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("with auth: status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestModelsOpenWhenAuthEnabled(t *testing.T) {
	h := newTestHubHTTP(NewBridgeRegistry(), &stubHandler{}, HTTPConfig{
		ValidateInbound: true,
		InboundAPIKeys:  map[string]struct{}{"secret-key": {}},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("models should be open: status = %d", rec.Code)
	}
}

func TestOpenAIPrefixedPaths(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{})
	reg.SetWorkReadyForTest(true)
	handler := &stubHandler{}
	h := newTestHubHTTP(reg, handler, HTTPConfig{ModelAlias: "local-ollama"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/openai/v1/models status = %d", rec.Code)
	}

	body := []byte(`{"model":"llama3"}`)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("/openai/v1/chat/completions status = %d", rec.Code)
	}
	if _, err := io.ReadAll(rec.Body); err != nil {
		t.Fatal(err)
	}
}

func TestLoadHTTPConfigDefaults(t *testing.T) {
	t.Setenv("PROXY_MODEL_ALIAS", "")
	t.Setenv("PROXY_VALIDATE_INBOUND", "")
	t.Setenv("PROXY_INBOUND_API_KEYS", "")
	cfg := loadHTTPConfig()
	if cfg.ModelAlias != "local-ollama" {
		t.Fatalf("alias = %q", cfg.ModelAlias)
	}
	if cfg.ValidateInbound {
		t.Fatal("expected validate false")
	}
}
