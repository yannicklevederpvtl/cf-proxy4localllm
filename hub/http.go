package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
	"github.com/google/uuid"
)

type HubHTTP struct {
	registry        *BridgeRegistry
	handler         chatHandler
	modelAlias      string
	validateInbound bool
	inboundKeys     map[string]struct{}
	metrics         *HubMetrics
}

func newHubHTTP(registry *BridgeRegistry, handler chatHandler, cfg HTTPConfig) *HubHTTP {
	return &HubHTTP{
		registry:        registry,
		handler:         handler,
		modelAlias:      cfg.ModelAlias,
		validateInbound: cfg.ValidateInbound,
		inboundKeys:     cfg.InboundAPIKeys,
		metrics:         NewHubMetrics(),
	}
}

func (h *HubHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health":
		h.handleHealth(w, r)
	case "/v1/models", "/openai/v1/models":
		h.handleModels(w, r)
	case "/v1/chat/completions", "/openai/v1/chat/completions":
		h.handleChatCompletions(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *HubHTTP) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.healthPayload())
}

func (h *HubHTTP) healthPayload() map[string]any {
	payload := map[string]any{
		"status":            "ok",
		"bridge_connected":  h.registry.IsConnected(),
		"bridge_work_ready": h.registry.HasWorkStream(),
		"uptime_seconds":    h.metrics.UptimeSeconds(),
		"requests_total":    h.metrics.Total(),
		"requests_failed":   h.metrics.Failed(),
	}
	if info := h.registry.Get(); info != nil {
		payload["bridge_registered_at"] = info.RegisteredAt.UTC().Format(time.RFC3339)
		payload["bridge_last_seen"] = info.LastSeenAt.UTC().Format(time.RFC3339)
		payload["bridge_ollama_url"] = info.OllamaBaseURL
		payload["bridge_default_model"] = info.DefaultModel
	}
	return payload
}

func (h *HubHTTP) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{
			{
				"id":           h.modelAlias,
				"object":       "model",
				"created":      time.Now().Unix(),
				"owned_by":     "proxy",
				"capabilities": []string{"chat"},
			},
		},
	})
}

func (h *HubHTTP) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if !h.registry.IsConnected() {
		writeBridgeUnavailable(w)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	requestID := uuid.New().String()
	if isStreamRequest(body) {
		h.handleStreamingChat(w, r, body, requestID)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamRequestTimeout)
	defer cancel()

	resp, err := h.handler.CompleteChat(ctx, &pb.ChatRequest{
		RequestId:      requestID,
		OpenaiJsonBody: body,
	})
	if err != nil {
		h.metrics.RecordRequest(true)
		status := chatErrorHTTPStatus(err)
		log.Printf("CompleteChat request_id=%s: %v", requestID, err)
		http.Error(w, err.Error(), status)
		return
	}
	h.metrics.RecordRequest(false)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(resp.GetOpenaiJsonBody()); err != nil {
		log.Printf("write chat response request_id=%s: %v", requestID, err)
	}
}

func (h *HubHTTP) authorize(r *http.Request) bool {
	if !h.validateInbound {
		return true
	}
	if len(h.inboundKeys) == 0 {
		return false
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	_, ok := h.inboundKeys[token]
	return ok
}

func writeBridgeUnavailable(w http.ResponseWriter) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"error": map[string]any{
			"message": "no bridge connected — start cf-proxy4localllm bridge on your laptop",
			"type":    "bridge_unavailable",
			"code":    503,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}
