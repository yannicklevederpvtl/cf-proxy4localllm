package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	pb "github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge/v1"
)

var (
	errNoBridgeWorkStream = errors.New("bridge work stream not connected")
	errBridgeNotReady     = errors.New("no bridge registered")
)

// BridgeInfo holds metadata from a successful Register call.
type BridgeInfo struct {
	OllamaBaseURL string
	DefaultModel  string
	RegisteredAt  time.Time
	LastSeenAt    time.Time
}

type pendingChat struct {
	respCh    chan *pb.ChatResponse
	streaming bool
}

// BridgeRegistry tracks a single connected bridge (v1 spike).
type BridgeRegistry struct {
	mu     sync.RWMutex
	bridge *BridgeInfo

	workMu     sync.Mutex
	workStream pb.LlmBridge_BridgeWorkServer
	workReadyTestHook bool

	pendingMu sync.Mutex
	pending   map[string]*pendingChat
}

func NewBridgeRegistry() *BridgeRegistry {
	return &BridgeRegistry{}
}

func (r *BridgeRegistry) Register(info BridgeInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	info.RegisteredAt = now
	info.LastSeenAt = now
	r.bridge = &info
}

func (r *BridgeRegistry) Get() *BridgeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.bridge == nil {
		return nil
	}
	copy := *r.bridge
	return &copy
}

func (r *BridgeRegistry) TouchLastSeen() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.bridge != nil {
		r.bridge.LastSeenAt = time.Now()
	}
}

func (r *BridgeRegistry) IsRegistered() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bridge != nil
}

// IsConnected reports whether a bridge is registered and its work stream is active.
func (r *BridgeRegistry) IsConnected() bool {
	return r.IsRegistered() && r.HasWorkStream()
}

// UpdateLastSeen records bridge activity (keepalive pong).
func (r *BridgeRegistry) UpdateLastSeen() {
	r.TouchLastSeen()
}

func (r *BridgeRegistry) HasWorkStream() bool {
	r.workMu.Lock()
	defer r.workMu.Unlock()
	return r.workStream != nil || r.workReadyTestHook
}

// SetWorkReadyForTest marks the registry as work-ready without a live gRPC stream.
func (r *BridgeRegistry) SetWorkReadyForTest(v bool) {
	r.workMu.Lock()
	defer r.workMu.Unlock()
	r.workReadyTestHook = v
}

func (r *BridgeRegistry) Deregister() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bridge = nil
}

func (r *BridgeRegistry) SetWorkStream(stream pb.LlmBridge_BridgeWorkServer) {
	r.workMu.Lock()
	defer r.workMu.Unlock()
	r.workStream = stream
}

func (r *BridgeRegistry) ClearWorkStream(stream pb.LlmBridge_BridgeWorkServer) {
	r.workMu.Lock()
	defer r.workMu.Unlock()
	if r.workStream == stream {
		r.workStream = nil
	}
}

func (r *BridgeRegistry) FulfillChatResponse(resp *pb.ChatResponse) {
	r.pendingMu.Lock()
	pc := r.pending[resp.GetRequestId()]
	r.pendingMu.Unlock()
	if pc == nil {
		return
	}
	pc.respCh <- resp
	if resp.GetStreamEnd() {
		r.pendingMu.Lock()
		delete(r.pending, resp.GetRequestId())
		r.pendingMu.Unlock()
		close(pc.respCh)
	}
}

func (r *BridgeRegistry) DispatchChat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	if !r.IsRegistered() {
		return nil, errBridgeNotReady
	}

	r.workMu.Lock()
	stream := r.workStream
	r.workMu.Unlock()
	if stream == nil {
		return nil, errNoBridgeWorkStream
	}

	pc := &pendingChat{respCh: make(chan *pb.ChatResponse, 1)}
	r.pendingMu.Lock()
	if r.pending == nil {
		r.pending = make(map[string]*pendingChat)
	}
	r.pending[req.GetRequestId()] = pc
	r.pendingMu.Unlock()

	defer func() {
		r.pendingMu.Lock()
		if entry := r.pending[req.GetRequestId()]; entry == pc {
			delete(r.pending, req.GetRequestId())
		}
		r.pendingMu.Unlock()
	}()

	if err := stream.Send(req); err != nil {
		return nil, fmt.Errorf("send chat request: %w", err)
	}

	select {
	case resp := <-pc.respCh:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *BridgeRegistry) DispatchStreamChat(ctx context.Context, req *pb.ChatRequest) (<-chan *pb.ChatResponse, error) {
	if !r.IsConnected() {
		return nil, errBridgeNotReady
	}

	r.workMu.Lock()
	stream := r.workStream
	r.workMu.Unlock()
	if stream == nil {
		return nil, errNoBridgeWorkStream
	}

	pc := &pendingChat{
		respCh:    make(chan *pb.ChatResponse, 32),
		streaming: true,
	}
	r.pendingMu.Lock()
	if r.pending == nil {
		r.pending = make(map[string]*pendingChat)
	}
	r.pending[req.GetRequestId()] = pc
	r.pendingMu.Unlock()

	if err := stream.Send(req); err != nil {
		r.pendingMu.Lock()
		delete(r.pending, req.GetRequestId())
		r.pendingMu.Unlock()
		close(pc.respCh)
		return nil, fmt.Errorf("send stream request: %w", err)
	}

	go func() {
		<-ctx.Done()
		r.pendingMu.Lock()
		if entry := r.pending[req.GetRequestId()]; entry == pc {
			delete(r.pending, req.GetRequestId())
			close(pc.respCh)
		}
		r.pendingMu.Unlock()
	}()

	return pc.respCh, nil
}
