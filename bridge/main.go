// Bridge: local daemon — outbound gRPC client to hub, HTTP to upstream LLM.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var bridgeVersion = "0.1.0" // overridden via -ldflags -X main.bridgeVersion=...

func main() {
	configPath := flag.String("config", "", "path to JSON config file (optional)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("cf-proxy4localllm-bridge", bridgeVersion)
		return
	}

	cfg := LoadConfig()
	if *configPath != "" {
		fc, err := loadFileConfig(*configPath)
		if err != nil {
			log.Fatalf("load config file: %v", err)
		}
		cfg = mergeFileConfig(fc, cfg)
	}
	log.Printf("cf-proxy4localllm bridge %s starting hub=%s tls=%v", bridgeVersion, cfg.HubAddr, cfg.UseTLS)

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
