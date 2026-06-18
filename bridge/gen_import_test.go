package main

import (
	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
)

// Compile-time check that generated gRPC stubs are importable.
func _genImportCheck() {
	_ = &pb.RegisterRequest{}
	_ = pb.NewLlmBridgeClient
}
