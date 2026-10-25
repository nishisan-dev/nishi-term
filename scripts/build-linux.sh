#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
pkg-config --exists gtk+-3.0 webkit2gtk-4.1 || {
  echo 'Instale libgtk-3-dev, libwebkit2gtk-4.1-dev e pkg-config antes de compilar.' >&2
  exit 1
}
wails_version="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2)"
go run "github.com/wailsapp/wails/v2/cmd/wails@${wails_version}" build -platform linux/amd64 -tags webkit2_41 -clean
