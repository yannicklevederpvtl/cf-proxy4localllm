package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HubAddr           string
	UseTLS            bool
	BridgeToken       string
	UpstreamBaseURL   string
	UpstreamAPIKey    string
	OllamaBaseURL     string // deprecated alias of UpstreamBaseURL
	DefaultModel      string
	ModelAlias        string
	KeepAliveInterval time.Duration
}

func LoadConfig() Config {
	upstream := envOr("UPSTREAM_BASE_URL", "")
	if upstream == "" {
		upstream = envOr("OLLAMA_BASE_URL", "http://127.0.0.1:11434/v1")
	}
	return Config{
		HubAddr:           envOr("HUB_GRPC_ADDR", "localhost:50051"),
		UseTLS:            envBool("HUB_GRPC_TLS", false),
		BridgeToken:       envOr("BRIDGE_TOKEN", "dev-token"),
		UpstreamBaseURL:   upstream,
		UpstreamAPIKey:    envOr("UPSTREAM_API_KEY", ""),
		OllamaBaseURL:     upstream,
		DefaultModel:      envOr("DEFAULT_MODEL", "llama3"),
		ModelAlias:        envOr("MODEL_ALIAS", "local-ollama"),
		KeepAliveInterval: envDuration("KEEPALIVE_INTERVAL", 5*time.Second),
	}
}

// usesOllamaExtensions is true for local Ollama (think=false, etc.).
func (c Config) upstreamBaseURL() string {
	if c.UpstreamBaseURL != "" {
		return c.UpstreamBaseURL
	}
	return c.OllamaBaseURL
}

func (c Config) usesOllamaExtensions() bool {
	return strings.Contains(strings.ToLower(c.upstreamBaseURL()), ":11434")
}

func (c Config) chatCompletionsURL() string {
	return strings.TrimSuffix(c.upstreamBaseURL(), "/") + "/chat/completions"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
