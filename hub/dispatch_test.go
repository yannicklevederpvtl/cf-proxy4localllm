package main

import (
	"context"
	"fmt"
	"io"
	"testing"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
	"google.golang.org/grpc/metadata"
)

type callbackWorkStream struct {
	ctx    context.Context
	onSend func(*pb.ChatRequest)
}

func (s *callbackWorkStream) Send(req *pb.ChatRequest) error {
	if s.onSend != nil {
		s.onSend(req)
	}
	return nil
}

func (s *callbackWorkStream) Recv() (*pb.ChatResponse, error) {
	return nil, io.EOF
}

func (s *callbackWorkStream) SetHeader(metadata.MD) error  { return nil }
func (s *callbackWorkStream) SendHeader(metadata.MD) error { return nil }
func (s *callbackWorkStream) SetTrailer(metadata.MD)       {}
func (s *callbackWorkStream) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
func (s *callbackWorkStream) SendMsg(any) error { return nil }
func (s *callbackWorkStream) RecvMsg(any) error { return io.EOF }

func TestDispatchChatRoundTrip(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{DefaultModel: "llama3"})

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
	resp, err := reg.DispatchChat(context.Background(), &pb.ChatRequest{
		RequestId:      "req-1",
		OpenaiJsonBody: body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.GetOpenaiJsonBody()) != string(body) {
		t.Fatalf("got %q", resp.GetOpenaiJsonBody())
	}
}

func TestDispatchChatNoWorkStream(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{})
	_, err := reg.DispatchChat(context.Background(), &pb.ChatRequest{RequestId: "x"})
	if err != errNoBridgeWorkStream {
		t.Fatalf("err = %v", err)
	}
}

func TestDispatchChatConcurrent(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{DefaultModel: "llama3"})

	stream := &callbackWorkStream{
		onSend: func(req *pb.ChatRequest) {
			reg.FulfillChatResponse(&pb.ChatResponse{
				RequestId:      req.GetRequestId(),
				OpenaiJsonBody: req.GetOpenaiJsonBody(),
			})
		},
	}
	reg.SetWorkStream(stream)

	const n = 10
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(id int) {
			body := []byte(`{"model":"llama3","messages":[]}`)
			_, err := reg.DispatchChat(context.Background(), &pb.ChatRequest{
				RequestId:      fmt.Sprintf("req-%d", id),
				OpenaiJsonBody: body,
			})
			errCh <- err
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent dispatch: %v", err)
		}
	}
}
