package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const ollamaRequestTimeout = 300 * time.Second

func resolveModelForUpstream(body []byte, cfg Config) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, nil
	}
	model, _ := payload["model"].(string)
	if model == "" || model == cfg.ModelAlias {
		payload["model"] = cfg.DefaultModel
	}
	if cfg.usesOllamaExtensions() {
		// qwen3 emits non-OpenAI "reasoning" chunks unless thinking is disabled
		payload["think"] = false
	}
	return json.Marshal(payload)
}

func applyUpstreamHeaders(req *http.Request, cfg Config) {
	req.Header.Set("Content-Type", "application/json")
	if cfg.UpstreamAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.UpstreamAPIKey)
	}
}

func requestModelFromBody(body []byte, cfg Config) string {
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Model != "" {
		return payload.Model
	}
	return cfg.ModelAlias
}

func sanitizeOpenAIJSON(body []byte, clientModel string) []byte {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	if clientModel != "" {
		payload["model"] = clientModel
	}
	if choices, ok := payload["choices"].([]any); ok {
		stripReasoningInChoices(choices)
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return out
}

func stripReasoningInChoices(choices []any) {
	for _, ch := range choices {
		choice, ok := ch.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"message", "delta"} {
			if m, ok := choice[key].(map[string]any); ok {
				delete(m, "reasoning")
			}
		}
	}
}

func sanitizeSSELine(line []byte, clientModel string) []byte {
	s := strings.TrimSpace(string(line))
	if !strings.HasPrefix(s, "data:") {
		return append(append([]byte(nil), line...), '\n')
	}
	data := strings.TrimSpace(strings.TrimPrefix(s, "data:"))
	if data == "[DONE]" {
		return []byte("data: [DONE]\n\n")
	}
	sanitized := sanitizeOpenAIJSON([]byte(data), clientModel)
	return append(append([]byte("data: "), sanitized...), '\n', '\n')
}

func forwardToOllama(ctx context.Context, cfg Config, body []byte) ([]byte, error) {
	clientModel := requestModelFromBody(body, cfg)
	body, err := resolveModelForUpstream(body, cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve model: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.chatCompletionsURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	applyUpstreamHeaders(req, cfg)

	client := &http.Client{Timeout: ollamaRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upstream response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, respBody)
	}
	return sanitizeOpenAIJSON(respBody, clientModel), nil
}

func forwardStreamToOllama(ctx context.Context, cfg Config, body []byte) (*http.Response, error) {
	body, err := resolveModelForUpstream(body, cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve model: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.chatCompletionsURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build upstream stream request: %w", err)
	}
	applyUpstreamHeaders(req, cfg)

	client := &http.Client{Timeout: ollamaRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream stream request: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, respBody)
	}
	return resp, nil
}

func streamOllamaSSE(ctx context.Context, cfg Config, body []byte, send func([]byte, bool) error) (int, error) {
	clientModel := requestModelFromBody(body, cfg)
	resp, err := forwardStreamToOllama(ctx, cfg, body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	count := 0
	sawDone := false
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		out := sanitizeSSELine(line, clientModel)
		if err := send(out, false); err != nil {
			return count, err
		}
		count++
		if bytes.Contains(line, []byte("[DONE]")) {
			sawDone = true
		}
	}
	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("read ollama stream: %w", err)
	}
	if !sawDone {
		if err := send([]byte("data: [DONE]\n\n"), false); err != nil {
			return count, err
		}
		count++
	}
	if err := send(nil, true); err != nil {
		return count, err
	}
	return count, nil
}
