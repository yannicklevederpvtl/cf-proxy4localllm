// Bridge: local daemon — outbound gRPC client to hub, HTTP to Ollama (Phase 0+).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := LoadConfig()
	log.Printf("cf-proxy4localllm bridge starting hub=%s tls=%v", cfg.HubAddr, cfg.UseTLS)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	attempt := 0
	for {
		if ctx.Err() != nil {
			log.Printf("bridge shutting down")
			return
		}

		err := runSession(ctx, cfg, func() { attempt = 0 })
		if ctx.Err() != nil {
			return
		}

		attempt++
		delay := reconnectDelay(attempt)
		log.Printf("hub session ended: %v; reconnecting in %s (attempt %d)", err, delay, attempt)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}
