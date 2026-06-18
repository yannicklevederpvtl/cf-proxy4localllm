#!/usr/bin/env python3
"""Smoke test: ai-models binding → GenAI proxy → cf-proxy4localllm hub → local Ollama."""

import json
import os
import sys
import urllib.error
import urllib.request

PROMPT = "Say exactly: feasibility verified"


def load_ai_credentials() -> dict:
    vcap = json.loads(os.environ.get("VCAP_SERVICES", "{}"))
    for key, instances in vcap.items():
        if "ai-models" in key or key == "ai-models":
            if not instances:
                continue
            creds = instances[0].get("credentials", {})
            if creds:
                return creds
    raise RuntimeError("no ai-models credentials in VCAP_SERVICES")


def chat(creds: dict) -> str:
    base = creds.get("api_base", creds.get("url", "")).rstrip("/")
    api_key = creds.get("api_key", creds.get("apiKey", ""))
    model = creds.get("model_name", creds.get("model", "local-ollama"))

    url = f"{base}/v1/chat/completions"
    body = json.dumps(
        {
            "model": model,
            "messages": [{"role": "user", "content": PROMPT}],
        }
    ).encode()

    req = urllib.request.Request(
        url,
        data=body,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}",
        },
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=300) as resp:
        data = json.loads(resp.read())

    choices = data.get("choices") or []
    if not choices:
        raise RuntimeError(f"no choices in response: {data}")
    content = (choices[0].get("message") or {}).get("content", "")
    if not content:
        raise RuntimeError(f"empty content: {data}")
    return content


def main() -> int:
    creds = load_ai_credentials()
    print("ai-models credentials keys:", sorted(creds.keys()))
    print("api_base:", creds.get("api_base", creds.get("url")))
    print("model:", creds.get("model_name", creds.get("model")))

    content = chat(creds)
    print("response:", content)
    if "feasibility" not in content.lower():
        print("FAIL: expected 'feasibility' in response", file=sys.stderr)
        return 1
    print("PASS: feasibility verified via ai-models binding")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
