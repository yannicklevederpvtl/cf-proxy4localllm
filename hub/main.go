// Hub: CF app — gRPC rendezvous + OpenAI HTTP face (Phase 0+).
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	registry := NewBridgeRegistry()
	token := bridgeTokenFromEnv()
	httpCfg := loadHTTPConfig()

	bridgeSrv := newBridgeServer(registry, token)
	grpcSrv := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			Time:              30 * time.Second,
			Timeout:           10 * time.Second,
		}),
	)
	pb.RegisterLlmBridgeServer(grpcSrv, bridgeSrv)
	reflection.Register(grpcSrv)

	httpSrv := &http.Server{
		Handler:      loggingMiddleware(newHubHTTP(registry, bridgeSrv, httpCfg)),
		ReadTimeout:  httpServerReadTimeout,
		WriteTimeout: httpServerWriteTimeout,
		IdleTimeout:  httpServerIdleTimeout,
	}

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	m := cmux.New(lis)
	grpcL := m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	httpL := m.Match(cmux.Any())

	go func() {
		if err := grpcSrv.Serve(grpcL); err != nil {
			log.Printf("gRPC server stopped: %v", err)
		}
	}()
	go func() {
		if err := httpSrv.Serve(httpL); err != nil && err != cmux.ErrServerClosed {
			log.Printf("HTTP server stopped: %v", err)
		}
	}()

	log.Printf("cf-proxy4localllm hub listening on %s (HTTP + gRPC via cmux, Phase 0 Ollama forwarding)", addr)
	if err := m.Serve(); err != nil {
		log.Fatal(err)
	}
}
