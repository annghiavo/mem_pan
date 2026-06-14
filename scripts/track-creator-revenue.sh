#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR/services/billing-service"
export GOCACHE="${GOCACHE:-/private/tmp/mem_pan_go_cache}"

go run ./cmd/track-creator-revenue "$@"
