package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
)

type chatHandler interface {
	CompleteChat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error)
	DispatchStream(ctx context.Context, req *pb.ChatRequest) (<-chan *pb.ChatResponse, error)
}

func isStreamRequest(body []byte) bool {
	var payload struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	return payload.Stream
}

func (h *HubHTTP) handleStreamingChat(w http.ResponseWriter, r *http.Request, body []byte, requestID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ctx, cancel := context.WithTimeout(r.Context(), upstreamRequestTimeout)
	defer cancel()

	chunks, err := h.handler.DispatchStream(ctx, &pb.ChatRequest{
		RequestId:      requestID,
		OpenaiJsonBody: body,
	})
	if err != nil {
		h.metrics.RecordRequest(true)
		log.Printf("DispatchStream request_id=%s: %v", requestID, err)
		fmt.Fprintf(w, "data: {\"error\":%q}\n\n", err.Error())
		flusher.Flush()
		return
	}

	wroteDone := false
	for chunk := range chunks {
		if chunk.GetStreamEnd() {
			break
		}
		line := chunk.GetOpenaiJsonBody()
		if len(line) == 0 {
			continue
		}
		if _, err := w.Write(line); err != nil {
			log.Printf("stream write request_id=%s: %v", requestID, err)
			return
		}
		if !bytes.HasSuffix(line, []byte("\n")) {
			_, _ = w.Write([]byte("\n"))
		}
		if bytes.Contains(line, []byte("[DONE]")) {
			wroteDone = true
		}
		flusher.Flush()
	}
	if !wroteDone {
		if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
			return
		}
		flusher.Flush()
	}
	h.metrics.RecordRequest(false)
}
