package main

import (
	"sync"
	"testing"
	"time"
)

func TestBridgeRegistryRace(t *testing.T) {
	reg := NewBridgeRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			reg.Register(BridgeInfo{
				DefaultModel:  "llama3",
				OllamaBaseURL: "http://127.0.0.1:11434/v1",
			})
			reg.UpdateLastSeen()
			_ = reg.IsConnected()
			_ = reg.IsRegistered()
			_ = reg.Get()
			if n%5 == 0 {
				reg.Deregister()
			}
		}(i)
	}
	wg.Wait()
}

func TestIsConnectedRequiresWorkStream(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{DefaultModel: "llama3"})
	if reg.IsConnected() {
		t.Fatal("connected without work stream")
	}
	reg.SetWorkReadyForTest(true)
	if !reg.IsConnected() {
		t.Fatal("expected connected with work stream")
	}
}

func TestUpdateLastSeen(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{DefaultModel: "llama3"})
	before := reg.Get().LastSeenAt
	time.Sleep(5 * time.Millisecond)
	reg.UpdateLastSeen()
	after := reg.Get().LastSeenAt
	if !after.After(before) {
		t.Fatalf("last_seen not updated: before=%v after=%v", before, after)
	}
}

func TestDeregisterClearsBridge(t *testing.T) {
	reg := NewBridgeRegistry()
	reg.Register(BridgeInfo{})
	reg.Deregister()
	if reg.IsRegistered() || reg.Get() != nil {
		t.Fatal("expected empty registry")
	}
}
