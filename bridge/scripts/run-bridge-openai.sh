#!/usr/bin/env bash
# Bridge → TPCF hub → OpenAI (or other cloud OpenAI-compatible API).
# Keeps the GenAI tile model alias (local-ollama) on CF; upstream model is DEFAULT_MODEL.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

if [[ -f "$ROOT/bridge/secrets.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/bridge/secrets.env"
  set +a
fi

export UPSTREAM_BASE_URL="${UPSTREAM_BASE_URL:-https://api.openai.com/v1}"
export DEFAULT_MODEL="${DEFAULT_MODEL:-gpt-4.1-mini}"
export MODEL_ALIAS="${MODEL_ALIAS:-local-ollama}"

if [[ -z "${UPSTREAM_API_KEY:-}" ]]; then
  echo "Set UPSTREAM_API_KEY in bridge/secrets.env (see secrets.env.example)" >&2
  exit 1
fi

exec "$ROOT/bridge/scripts/run-bridge-tpcf.sh"
