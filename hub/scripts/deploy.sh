#!/usr/bin/env bash
# Push hub to Cloud Foundry. Vendors gen/llmbridge stubs then cf push.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VARS_FILE="${1:-$ROOT/vars.yml}"

if [[ ! -f "$VARS_FILE" ]]; then
  echo "Missing vars file: $VARS_FILE" >&2
  echo "Copy vars.yml.example to vars.yml and set cf_domain + bridge_token." >&2
  exit 1
fi

cd "$ROOT"
echo "==> go mod vendor (includes gen/llmbridge via replace)"
go mod vendor
go mod tidy
go test -c -o /dev/null .

echo "==> cf push"
cf push -f manifest.yml --vars-file "$VARS_FILE"
