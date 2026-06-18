# CF Local LLM Bridge

macOS app for [cf-proxy4localllm](../README.md). Configure your laptop bridge, connect to the TPCF hub, and view logs — without running shell scripts in Terminal.

**Download:** [GitHub Releases](https://github.com/yannicklevederpvtl/cf-proxy4localllm/releases) — look for tags `desktop-v*` (e.g. `desktop-v0.1.0`) and download `CF-Local-LLM-Bridge-*-macOS.dmg`.

The app checks for updates against those same release tags.

## Requirements

- macOS 14+ (Apple Silicon or Intel)
- A deployed **hub** on TPCF (see main [README](../README.md))
- Local **Ollama** or another OpenAI-compatible API on your Mac

## Install

1. Download the latest `CF-Local-LLM-Bridge-*-macOS.dmg` from [Releases](https://github.com/yannicklevederpvtl/cf-proxy4localllm/releases).
2. Open the DMG and drag **CF Local LLM Bridge** to Applications.
3. First launch: if macOS blocks the app, open **System Settings → Privacy & Security** and allow it, or use a signed/notarized build from Releases.

## Quick start

1. Open **CF Local LLM Bridge**.
2. Go to **Settings**.
3. Set **Hub gRPC address** and **HTTP base URL** (defaults match a typical TPCF deploy).
4. **Bridge token** — paste the same value as `bridge_token` in your hub deploy vars, or use **Load bridge token from hub/vars.yml** if you have the repo locally.
5. Choose **Ollama** or **OpenAI** upstream profile and model.
6. Click **Save** in the toolbar (or press **⌘S**). Click **Always Allow** if Keychain prompts.
7. Open **Connection** and click **Connect**.

The menu bar shows a white **arch** icon: bright when connected, dim when disconnected.

## Settings storage

| Item | Location |
|------|----------|
| Hub URLs, models, preferences | `~/Library/Application Support/CFLocalLLMBridge/config.json` |
| Bridge token, upstream API key | macOS Keychain (service `CFLocalLLMBridge`) |

Secrets are never written to `config.json`. Do not commit `hub/vars.yml` or `bridge/secrets.env`.

## UI overview

| Area | Purpose |
|------|---------|
| **Connection** | Connect / Disconnect, status, open hub health URL |
| **Settings** | Hub, token, upstream profile (Ollama / OpenAI / Custom) |
| **Logs** | Bridge log tail, filter keepalive, copy |
| **Menu bar** | Status, Connect / Disconnect, Open Window, Quit |

Closing the window can hide to the menu bar while the bridge keeps running (toggle in Settings).

## Troubleshooting

| Issue | What to do |
|-------|------------|
| Keychain prompts repeatedly | Click **Always Allow** once; in Settings click **Save** again to re-store secrets |
| Hub rejects token | Token must match hub `bridge_token` / `vars.yml` |
| Cannot reach hub | Check gRPC address, **Use TLS**, VPN, and hub is running (`cf apps`) |
| Upstream errors | Start Ollama, or set OpenAI API key for OpenAI profile |
| Chat fails on agent | Hub `/health` should show `bridge_connected: true` |

More hub/bridge troubleshooting: [main README](../README.md#troubleshooting).

## Releasing (maintainers)

Desktop app **source and binaries are not stored in git**. Builds are published via **GitHub Releases** only.

GitHub always shows **Source code (zip/tar.gz)** on releases; that cannot be removed. Root `.gitattributes` keeps those archives to install docs only — **download the `.dmg` asset**, not Source code.

From a development clone (with `desktop-app/swift/` source):

```bash
chmod +x desktop-app/scripts/*.sh
./desktop-app/scripts/package-dmg.sh   # Swift universal .app + DMG
gh release create desktop-v0.1.0 \
  --title "CF Local LLM Bridge 0.1.0" \
  --generate-notes \
  desktop-app/dist/release/CF-Local-LLM-Bridge-0.1.0-macOS.dmg
```

Local test without DMG: `./desktop-app/scripts/build-swift.sh` then `open desktop-app/dist/CF\ Local\ LLM\ Bridge.app`

Signed/notarized builds: use your Apple Developer credentials (see `DISTRIBUTION.md` in the dev tree).

Pushing a `desktop-v*` tag may create a draft release via CI; **attach the DMG** if the workflow did not upload artifacts.
