package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMergeFileConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bridge.json")
	if err := os.WriteFile(path, []byte(`{
		"hub_grpc_addr": "hub.example:443",
		"hub_grpc_tls": true,
		"bridge_token": "secret",
		"upstream_base_url": "https://api.openai.com/v1",
		"upstream_api_key": "sk-test",
		"default_model": "gpt-4.1-mini",
		"model_alias": "local-ollama",
		"keepalive_interval": "10s"
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fc, err := loadFileConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := mergeFileConfig(fc, Config{DefaultModel: "llama3"})
	if cfg.HubAddr != "hub.example:443" || !cfg.UseTLS {
		t.Fatalf("hub = %+v", cfg)
	}
	if cfg.DefaultModel != "gpt-4.1-mini" {
		t.Fatalf("model = %q", cfg.DefaultModel)
	}
	if cfg.KeepAliveInterval != 10*time.Second {
		t.Fatalf("keepalive = %v", cfg.KeepAliveInterval)
	}
}
