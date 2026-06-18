#!/usr/bin/env bash
# Run the laptop bridge against TPCF hub. Keep this terminal open while using the agent.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT/bridge"

: "${HUB_GRPC_ADDR:=cf-proxy4localllm-grpc.apps.tas02.ylhomelab.com:443}"
: "${HUB_GRPC_TLS:=true}"
: "${UPSTREAM_BASE_URL:=${OLLAMA_BASE_URL:-http://127.0.0.1:11434/v1}}"
: "${DEFAULT_MODEL:=qwen3:8b}"
: "${MODEL_ALIAS:=local-ollama}"
: "${KEEPALIVE_INTERVAL:=5s}"
: "${UPSTREAM_API_KEY:=}"

if [[ -z "${BRIDGE_TOKEN:-}" ]]; then
  if [[ -f "$ROOT/hub/vars.yml" ]]; then
    BRIDGE_TOKEN="$(grep bridge_token "$ROOT/hub/vars.yml" | awk '{print $2}')"
  fi
fi
if [[ -z "${BRIDGE_TOKEN:-}" ]]; then
  echo "Set BRIDGE_TOKEN or add bridge_token to hub/vars.yml" >&2
  exit 1
fi

go build -o /tmp/cf-proxy4localllm-bridge .
echo "Starting bridge → $HUB_GRPC_ADDR upstream=$UPSTREAM_BASE_URL model=$DEFAULT_MODEL (keepalive=$KEEPALIVE_INTERVAL)"
exec env \
  HUB_GRPC_ADDR="$HUB_GRPC_ADDR" \
  HUB_GRPC_TLS="$HUB_GRPC_TLS" \
  BRIDGE_TOKEN="$BRIDGE_TOKEN" \
  UPSTREAM_BASE_URL="$UPSTREAM_BASE_URL" \
  UPSTREAM_API_KEY="$UPSTREAM_API_KEY" \
  DEFAULT_MODEL="$DEFAULT_MODEL" \
  MODEL_ALIAS="$MODEL_ALIAS" \
  KEEPALIVE_INTERVAL="$KEEPALIVE_INTERVAL" \
  /tmp/cf-proxy4localllm-bridge
