#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
export DEMO_MODE=true
export ADDR=127.0.0.1:8080
echo 'RevenueLeak OS demo: http://127.0.0.1:8080'
exec go run ./cmd/server
