package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func dial(ctx context.Context, cfg Config) (*grpc.ClientConn, error) {
	var creds credentials.TransportCredentials
	if cfg.UseTLS {
		creds = credentials.NewTLS(nil)
	} else {
		creds = insecure.NewCredentials()
	}

	return grpc.NewClient(cfg.HubAddr,
		grpc.WithTransportCredentials(creds),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                5 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	)
}

func runSession(ctx context.Context, cfg Config, onRegistered func()) error {
	conn, err := dial(ctx, cfg)
	if err != nil {
		return fmt.Errorf("dial hub: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	client := pb.NewLlmBridgeClient(conn)

	regCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	resp, err := client.Register(regCtx, &pb.RegisterRequest{
		BridgeToken:   cfg.BridgeToken,
		OllamaBaseUrl: cfg.UpstreamBaseURL,
		DefaultModel:  cfg.DefaultModel,
	})
	cancel()
	if err != nil {
		return fmt.Errorf("register rpc: %w", err)
	}
	if !resp.GetAccepted() {
		return fmt.Errorf("register rejected: %s", resp.GetMessage())
	}
	log.Printf("registered with hub: %s (upstream=%s model=%s)", resp.GetMessage(), cfg.UpstreamBaseURL, cfg.DefaultModel)
	connectedAt := time.Now()
	log.Printf("[bridge] connected to hub at %v", connectedAt)
	if onRegistered != nil {
		onRegistered()
	}

	kaCtx, kaCancel := context.WithCancel(ctx)
	defer kaCancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- runKeepAlive(kaCtx, client, cfg.KeepAliveInterval, connectedAt)
	}()
	go func() {
		errCh <- runBridgeWork(ctx, client, cfg)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		kaCancel()
		if err == io.EOF {
			return nil
		}
		return err
	}
}

func runKeepAlive(ctx context.Context, client pb.LlmBridgeClient, interval time.Duration, connectedAt time.Time) error {
	stream, err := client.KeepAlive(ctx)
	if err != nil {
		return fmt.Errorf("keepalive stream: %w", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pingAt := time.Now()
			log.Printf("[keepalive] ping sent at %v", pingAt)
			ts := pingAt.UnixMilli()
			if err := stream.Send(&pb.KeepAlivePing{TimestampMs: ts}); err != nil {
				return fmt.Errorf("keepalive send: %w", err)
			}
			if _, err := stream.Recv(); err != nil {
				return fmt.Errorf("keepalive recv: %w", err)
			}
			log.Printf("[keepalive] pong received, RTT=%v", time.Since(pingAt))
			log.Printf("[bridge] connection alive for %v", time.Since(connectedAt))
		}
	}
}
