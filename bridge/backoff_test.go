package main

import (
	"testing"
	"time"
)

func TestReconnectDelayBase(t *testing.T) {
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{10, 60 * time.Second},
	}
	for _, tc := range tests {
		if got := reconnectDelayBase(tc.attempt); got != tc.expected {
			t.Fatalf("attempt %d: got %v want %v", tc.attempt, got, tc.expected)
		}
	}
}

func TestReconnectDelayJitterBounds(t *testing.T) {
	for attempt := 1; attempt <= 5; attempt++ {
		base := reconnectDelayBase(attempt)
		min := time.Duration(float64(base) * 0.8)
		max := time.Duration(float64(base) * 1.2)
		for i := 0; i < 50; i++ {
			got := reconnectDelay(attempt)
			if got < min || got > max {
				t.Fatalf("attempt %d: got %v outside [%v, %v]", attempt, got, min, max)
			}
		}
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("HUB_GRPC_ADDR", "")
	t.Setenv("BRIDGE_TOKEN", "")
	cfg := LoadConfig()
	if cfg.HubAddr != "localhost:50051" {
		t.Fatalf("hub addr = %q", cfg.HubAddr)
	}
	if cfg.BridgeToken != "dev-token" {
		t.Fatalf("token = %q", cfg.BridgeToken)
	}
	if cfg.KeepAliveInterval != 5*time.Second {
		t.Fatalf("keepalive = %v", cfg.KeepAliveInterval)
	}
}
