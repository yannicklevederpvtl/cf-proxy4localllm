package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// FileConfig is optional JSON config for CF Local LLM Bridge desktop app.
// Env vars still override when set.
type FileConfig struct {
	HubGRPCAddr       string `json:"hub_grpc_addr"`
	HubGRPCTLS        bool   `json:"hub_grpc_tls"`
	BridgeToken       string `json:"bridge_token"`
	UpstreamBaseURL   string `json:"upstream_base_url"`
	UpstreamAPIKey    string `json:"upstream_api_key"`
	DefaultModel      string `json:"default_model"`
	ModelAlias        string `json:"model_alias"`
	KeepAliveInterval string `json:"keepalive_interval"`
}

func loadFileConfig(path string) (FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileConfig{}, err
	}
	var fc FileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return FileConfig{}, fmt.Errorf("parse config: %w", err)
	}
	return fc, nil
}

func mergeFileConfig(fc FileConfig, fallback Config) Config {
	cfg := fallback
	if fc.HubGRPCAddr != "" {
		cfg.HubAddr = fc.HubGRPCAddr
	}
	cfg.UseTLS = fc.HubGRPCTLS
	if fc.BridgeToken != "" {
		cfg.BridgeToken = fc.BridgeToken
	}
	if fc.UpstreamBaseURL != "" {
		cfg.UpstreamBaseURL = fc.UpstreamBaseURL
		cfg.OllamaBaseURL = fc.UpstreamBaseURL
	}
	if fc.UpstreamAPIKey != "" {
		cfg.UpstreamAPIKey = fc.UpstreamAPIKey
	}
	if fc.DefaultModel != "" {
		cfg.DefaultModel = fc.DefaultModel
	}
	if fc.ModelAlias != "" {
		cfg.ModelAlias = fc.ModelAlias
	}
	if fc.KeepAliveInterval != "" {
		if d, err := time.ParseDuration(fc.KeepAliveInterval); err == nil {
			cfg.KeepAliveInterval = d
		}
	}
	return cfg
}
