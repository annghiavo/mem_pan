#!/bin/sh
# Seed stats-service with initial events by posting directly to its push endpoint.
# Bypasses Pub/Sub — useful for backfilling data that was missed before Pub/Sub was set up.
#
# Usage:
#   docker compose run --rm pubsub-setup sh /scripts/seed-events.sh \
#     <user_id> <username> <email> <deck_id> <deck_name>
#
# Example:
#   docker compose run --rm pubsub-setup sh /scripts/seed-events.sh \
#     330fb773-e071-4a92-929a-33b3430c5446 anvo user@example.com \
#     838f8f13-b720-4239-b6c1-0dc2b2572282 "English 5 4U"

STATS_HOST="${STATS_SERVICE_HOST:-stats-service:8084}"
SECRET="${PUBSUB_PUSH_SECRET:-dev-secret}"
ENDPOINT="http://${STATS_HOST}/internal/pubsub?token=${SECRET}"
NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG_ID=0

# push_event <event_type> <payload_json>
# Builds the double-base64 structure the stats-service push handler expects:
#   message.data = base64( {"event_type":..., "data": base64(<payload_json>)} )
push_event() {
  EVENT_TYPE="$1"
  PAYLOAD="$2"
  MSG_ID=$((MSG_ID + 1))

  INNER_B64=$(printf '%s' "$PAYLOAD" | base64 | tr -d '\n')
  ENVELOPE="{\"event_type\":\"${EVENT_TYPE}\",\"data\":\"${INNER_B64}\"}"
  OUTER_B64=$(printf '%s' "$ENVELOPE" | base64 | tr -d '\n')

  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ENDPOINT" \
    -H "Content-Type: application/json" \
    -d "{
      \"message\": {
        \"data\": \"${OUTER_B64}\",
        \"messageId\": \"seed-${MSG_ID}\",
        \"publishTime\": \"${NOW}\"
      },
      \"subscription\": \"projects/local/subscriptions/stats-seed\"
    }")

  if [ "$HTTP_CODE" = "204" ]; then
    echo "  OK  ${EVENT_TYPE}"
  else
    echo "  FAIL ${EVENT_TYPE} (HTTP ${HTTP_CODE})"
  fi
}

USER_ID="${1:?usage: seed-events.sh <user_id> <username> <email> <deck_id> <deck_name>}"
USERNAME="${2:?missing username}"
EMAIL="${3:?missing email}"
DECK_ID="${4:?missing deck_id}"
DECK_NAME="${5:?missing deck_name}"

echo "Seeding stats-service at ${ENDPOINT}..."

push_event "user.registered" \
  "{\"user_id\":\"${USER_ID}\",\"username\":\"${USERNAME}\",\"email\":\"${EMAIL}\",\"avatar_url\":\"\",\"created_at\":\"${NOW}\"}"

push_event "deck.created" \
  "{\"deck_id\":\"${DECK_ID}\",\"user_id\":\"${USER_ID}\",\"deck_name\":\"${DECK_NAME}\",\"created_at\":\"${NOW}\"}"

echo "Done."
