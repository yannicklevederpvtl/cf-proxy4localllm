package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
)

type streamSender func(chunk []byte, done bool) error

func runBridgeWork(ctx context.Context, client pb.LlmBridgeClient, cfg Config) error {
	stream, err := client.BridgeWork(ctx)
	if err != nil {
		return fmt.Errorf("bridge work stream: %w", err)
	}
	log.Printf("bridge work stream connected to hub")

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("bridge work recv: %w", err)
		}

		if isStreamRequest(req.GetOpenaiJsonBody()) {
			send := func(chunk []byte, done bool) error {
				return stream.Send(&pb.ChatResponse{
					RequestId:      req.GetRequestId(),
					OpenaiJsonBody: chunk,
					StreamEnd:      done,
				})
			}
			if err := handleStreamChatRequest(ctx, cfg, req, send); err != nil {
				log.Printf("stream chat request_id=%s: %v", req.GetRequestId(), err)
			}
			continue
		}

		resp, err := handleChatRequest(ctx, cfg, req)
		if err != nil {
			log.Printf("chat request_id=%s: %v", req.GetRequestId(), err)
		}
		if sendErr := stream.Send(resp); sendErr != nil {
			return fmt.Errorf("bridge work send request_id=%s: %w", req.GetRequestId(), sendErr)
		}
	}
}

func handleChatRequest(ctx context.Context, cfg Config, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	body, err := ensureNonStreamingBody(req.GetOpenaiJsonBody())
	if err != nil {
		errBody, _ := json.Marshal(map[string]string{"error": err.Error()})
		return &pb.ChatResponse{
			RequestId:      req.GetRequestId(),
			OpenaiJsonBody: errBody,
		}, err
	}

	body, err = forwardToOllama(ctx, cfg, body)
	if err != nil {
		errBody, _ := json.Marshal(map[string]string{"error": err.Error()})
		return &pb.ChatResponse{
			RequestId:      req.GetRequestId(),
			OpenaiJsonBody: errBody,
		}, err
	}
	log.Printf("chat request_id=%s completed (%d bytes)", req.GetRequestId(), len(body))
	return &pb.ChatResponse{
		RequestId:      req.GetRequestId(),
		OpenaiJsonBody: body,
	}, nil
}

func handleStreamChatRequest(ctx context.Context, cfg Config, req *pb.ChatRequest, send streamSender) error {
	body, err := ensureStreamingBody(req.GetOpenaiJsonBody())
	if err != nil {
		errBody, _ := json.Marshal(map[string]string{"error": err.Error()})
		_ = send(errBody, true)
		return err
	}

	chunkCount, err := streamOllamaSSE(ctx, cfg, body, send)
	if err != nil {
		errBody, _ := json.Marshal(map[string]string{"error": err.Error()})
		_ = send(errBody, true)
		return err
	}
	log.Printf("stream request_id=%s completed (%d chunks)", req.GetRequestId(), chunkCount)
	return nil
}

func ensureStreamingBody(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, nil
	}
	payload["stream"] = true
	return json.Marshal(payload)
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

func ensureNonStreamingBody(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, nil
	}
	payload["stream"] = false
	return json.Marshal(payload)
}

func isOpenAIChatCompletion(body []byte) bool {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	return len(resp.Choices) > 0
}
