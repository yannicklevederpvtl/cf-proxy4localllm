package main

import (
	"os"
	"strconv"
	"strings"
)

type HTTPConfig struct {
	ModelAlias     string
	ValidateInbound bool
	InboundAPIKeys map[string]struct{}
}

func loadHTTPConfig() HTTPConfig {
	keys := make(map[string]struct{})
	for _, k := range strings.Split(os.Getenv("PROXY_INBOUND_API_KEYS"), ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			keys[k] = struct{}{}
		}
	}

	alias := os.Getenv("PROXY_MODEL_ALIAS")
	if alias == "" {
		alias = "local-ollama"
	}

	return HTTPConfig{
		ModelAlias:      alias,
		ValidateInbound: envBool("PROXY_VALIDATE_INBOUND", false),
		InboundAPIKeys:  keys,
	}
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
