#!/usr/bin/env bash
# Reconstruit bin/mulcv-depot.exe de facon reproductible (meme version de Go que la CI : 1.24.7).
set -euo pipefail
cd "$(dirname "$0")/src"
go test ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o ../bin/mulcv-depot.exe .
sha256sum ../bin/mulcv-depot.exe
