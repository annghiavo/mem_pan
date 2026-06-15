#!/usr/bin/env bash
# demo-revshare-calculate.sh — gọi trực tiếp HTTP POST /internal/revshare/calculate
# để test tính revenue share và đồng bộ sang billing-service.
#
# Ví dụ:
#   POOL_MONTH=2026-05 GROSS_AMOUNT_VND=500000 ./scripts/demo/demo-revshare-calculate.sh
# Hoặc:
#   STUDY_SERVICE_URL=http://localhost:8082 CRON_SECRET=local-dev-cron-secret \
#   POOL_MONTH=2026-05 GROSS_AMOUNT_VND=500000 ./scripts/demo/demo-revshare-calculate.sh

set -euo pipefail

STUDY_SERVICE_URL="${STUDY_SERVICE_URL:-http://localhost:8082}"
CRON_SECRET="${CRON_SECRET:-local-dev-cron-secret}"
POOL_MONTH="${POOL_MONTH:-$(date -u -v-1m +%Y-%m 2>/dev/null || python3 - <<'PY'
from datetime import datetime, timezone
now = datetime.now(timezone.utc)
year = now.year
month = now.month - 1
if month == 0:
    month = 12
    year -= 1
print(f"{year:04d}-{month:02d}")
PY
)}"
GROSS_AMOUNT_VND="${GROSS_AMOUNT_VND:-0}"
POOL_RATE="${POOL_RATE:-0.5}"
MIN_LEARNERS="${MIN_LEARNERS:-1}"
CREATOR_CAP_RATE="${CREATOR_CAP_RATE:-0.2}"

URI="${STUDY_SERVICE_URL%/}/internal/revshare/calculate"
BODY="$(printf '{"pool_month":"%s","gross_amount_vnd":%s,"pool_rate":%s,"min_learners":%s,"creator_cap_rate":%s}' \
  "$POOL_MONTH" "$GROSS_AMOUNT_VND" "$POOL_RATE" "$MIN_LEARNERS" "$CREATOR_CAP_RATE")"

echo "=== DEMO CREATOR REVSHARE CALCULATION ==="
echo "Endpoint: $URI"
echo "Payload: $BODY"
echo "----------------------------------------"

response=$(curl -s -w "\n%{http_code}" -X POST "$URI" \
  -H "Content-Type: application/json" \
  -H "X-Cron-Secret: $CRON_SECRET" \
  --data "$BODY" \
  --max-time 1800)

http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')

echo "HTTP Status Code: $http_code"
if [ "$http_code" -eq 200 ]; then
  echo "Revshare calculation THÀNH CÔNG"
  if command -v jq >/dev/null 2>&1; then
    echo "$body" | jq .
  else
    echo "$body"
  fi
  echo ""
  echo "Kiểm tra billing-service sau khi sync:"
  echo "  bash scripts/track-creator-revenue.sh"
else
  echo "Revshare calculation THẤT BẠI"
  echo "$body"
fi
