#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STUDY_ENV_FILE="${STUDY_ENV_FILE:-$ROOT_DIR/services/study-service/app.env}"
BILLING_ENV_FILE="${BILLING_ENV_FILE:-$ROOT_DIR/services/billing-service/app.env}"

read_env_value() {
  local file="$1"
  local key="$2"
  awk -F= -v search_key="$key" '$1 == search_key { sub(/^[^=]*=/, "", $0); print $0; exit }' "$file"
}

STUDY_DATABASE_URL="${STUDY_DATABASE_URL:-$(read_env_value "$STUDY_ENV_FILE" DATABASE_URL)}"
BILLING_DATABASE_URL="${BILLING_DATABASE_URL:-$(read_env_value "$BILLING_ENV_FILE" DATABASE_URL)}"

if [[ -z "${STUDY_DATABASE_URL:-}" ]]; then
  echo "STUDY_DATABASE_URL is required" >&2
  exit 1
fi
if [[ -z "${BILLING_DATABASE_URL:-}" ]]; then
  echo "BILLING_DATABASE_URL is required" >&2
  exit 1
fi

cd "$ROOT_DIR/services/billing-service"
export GOCACHE="${GOCACHE:-/private/tmp/mem_pan_go_cache}"
export STUDY_DATABASE_URL
export BILLING_DATABASE_URL

go run ./cmd/sync-revshare-from-study "$@"
