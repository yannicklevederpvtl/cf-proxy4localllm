package main

import (
	"context"
	"testing"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
)

func TestRegisterRejectsInvalidToken(t *testing.T) {
	reg := NewBridgeRegistry()
	srv := newBridgeServer(reg, "secret")

	resp, err := srv.Register(context.Background(), &pb.RegisterRequest{
		BridgeToken:   "wrong",
		OllamaBaseUrl: "http://127.0.0.1:11434/v1",
		DefaultModel:  "llama3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetAccepted() {
		t.Fatal("expected rejected registration")
	}
	if reg.IsRegistered() {
		t.Fatal("registry should stay empty")
	}
}

func TestRegisterAcceptsValidToken(t *testing.T) {
	reg := NewBridgeRegistry()
	srv := newBridgeServer(reg, "secret")

	resp, err := srv.Register(context.Background(), &pb.RegisterRequest{
		BridgeToken:   "secret",
		OllamaBaseUrl: "http://127.0.0.1:11434/v1",
		DefaultModel:  "llama3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetAccepted() {
		t.Fatalf("expected accepted: %s", resp.GetMessage())
	}
	if !reg.IsRegistered() {
		t.Fatal("registry should have bridge")
	}
	info := reg.Get()
	if info.DefaultModel != "llama3" {
		t.Fatalf("model = %q", info.DefaultModel)
	}
}

func TestCompleteChatDispatchesToBridge(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{DefaultModel: "llama3"})
	srv := newBridgeServer(reg, "secret")

	stream := &callbackWorkStream{
		onSend: func(req *pb.ChatRequest) {
			reg.FulfillChatResponse(&pb.ChatResponse{
				RequestId:      req.GetRequestId(),
				OpenaiJsonBody: req.GetOpenaiJsonBody(),
			})
		},
	}
	reg.SetWorkStream(stream)

	body := []byte(`{"model":"llama3","messages":[]}`)
	resp, err := srv.CompleteChat(context.Background(), &pb.ChatRequest{
		RequestId:      "req-1",
		OpenaiJsonBody: body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.GetOpenaiJsonBody()) != string(body) {
		t.Fatalf("echo mismatch: %q", resp.GetOpenaiJsonBody())
	}
}
