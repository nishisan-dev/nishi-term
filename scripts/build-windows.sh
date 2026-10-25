#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Use the CLI version pinned in go.mod. No machine-wide installation required.
wails_version="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2)"
go run "github.com/wailsapp/wails/v2/cmd/wails@${wails_version}" build -platform windows/amd64 -webview2 error -installscope user -clean
