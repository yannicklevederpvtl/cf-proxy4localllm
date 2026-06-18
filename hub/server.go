package main

import (
	"context"
	"io"
	"log"
	"os"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type bridgeServer struct {
	pb.UnimplementedLlmBridgeServer
	registry    *BridgeRegistry
	bridgeToken string
}

func newBridgeServer(registry *BridgeRegistry, bridgeToken string) *bridgeServer {
	return &bridgeServer{
		registry:    registry,
		bridgeToken: bridgeToken,
	}
}

func (s *bridgeServer) Register(_ context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if s.bridgeToken == "" {
		return &pb.RegisterResponse{
			Accepted: false,
			Message:  "hub BRIDGE_TOKEN is not configured",
		}, nil
	}
	if req.GetBridgeToken() != s.bridgeToken {
		return &pb.RegisterResponse{
			Accepted: false,
			Message:  "invalid bridge_token",
		}, nil
	}

	ollamaURL := req.GetOllamaBaseUrl()
	if ollamaURL == "" {
		ollamaURL = "http://127.0.0.1:11434/v1"
	}
	model := req.GetDefaultModel()
	if model == "" {
		model = "llama3"
	}

	s.registry.Register(BridgeInfo{
		OllamaBaseURL: ollamaURL,
		DefaultModel:  model,
	})

	log.Printf("bridge registered ollama=%s model=%s", ollamaURL, model)
	return &pb.RegisterResponse{
		Accepted: true,
		Message:  "registered",
	}, nil
}

func (s *bridgeServer) CompleteChat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	return s.registry.DispatchChat(ctx, req)
}

func (s *bridgeServer) DispatchStream(ctx context.Context, req *pb.ChatRequest) (<-chan *pb.ChatResponse, error) {
	return s.registry.DispatchStreamChat(ctx, req)
}

func (s *bridgeServer) StreamChat(req *pb.ChatRequest, stream pb.LlmBridge_StreamChatServer) error {
	chunks, err := s.registry.DispatchStreamChat(stream.Context(), req)
	if err != nil {
		return err
	}
	for chunk := range chunks {
		if chunk.GetStreamEnd() {
			break
		}
		if err := stream.Send(&pb.ChatChunk{SseLine: chunk.GetOpenaiJsonBody()}); err != nil {
			return err
		}
	}
	return nil
}

func (s *bridgeServer) BridgeWork(stream pb.LlmBridge_BridgeWorkServer) error {
	if !s.registry.IsRegistered() {
		return status.Error(codes.FailedPrecondition, "bridge must Register before BridgeWork")
	}
	s.registry.SetWorkStream(stream)
	defer func() {
		s.registry.ClearWorkStream(stream)
		s.registry.Deregister()
		log.Printf("bridge work stream disconnected")
	}()
	log.Printf("bridge work stream connected")

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		s.registry.FulfillChatResponse(resp)
	}
}

func (s *bridgeServer) KeepAlive(stream pb.LlmBridge_KeepAliveServer) error {
	defer func() {
		if !s.registry.HasWorkStream() {
			s.registry.Deregister()
			log.Printf("bridge keepalive disconnected")
		}
	}()
	for {
		ping, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		s.registry.UpdateLastSeen()
		if err := stream.Send(&pb.KeepAlivePong{TimestampMs: ping.GetTimestampMs()}); err != nil {
			return err
		}
	}
}

func bridgeTokenFromEnv() string {
	if token := os.Getenv("BRIDGE_TOKEN"); token != "" {
		return token
	}
	log.Printf("warning: BRIDGE_TOKEN unset — using dev-token for local Phase 0 only")
	return "dev-token"
}
