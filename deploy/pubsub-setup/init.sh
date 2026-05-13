#!/bin/sh
set -e

EMULATOR="http://${PUBSUB_EMULATOR_HOST:-pubsub-emulator:8085}"
PROJECT="${PUBSUB_PROJECT_ID:-local-dev}"
SECRET="${PUBSUB_PUSH_SECRET:-dev-secret}"
STATS_HOST="${STATS_SERVICE_HOST:-stats-service:8084}"
PUSH_URL="http://${STATS_HOST}/internal/pubsub?token=${SECRET}"

echo "Waiting for Pub/Sub emulator at ${EMULATOR}..."
until curl -sf -o /dev/null "${EMULATOR}/v1/projects/${PROJECT}/topics"; do
  sleep 2
done

for TOPIC in user-events deck-events study-events; do
  echo "Creating topic ${TOPIC}..."
  curl -s -X PUT "${EMULATOR}/v1/projects/${PROJECT}/topics/${TOPIC}" \
    -H "Content-Type: application/json" \
    -d '{}' > /dev/null || true
done

declare_sub() {
  TOPIC="$1"
  SUB="$2"
  echo "Creating subscription ${SUB} -> ${PUSH_URL}..."
  curl -s -X PUT "${EMULATOR}/v1/projects/${PROJECT}/subscriptions/${SUB}" \
    -H "Content-Type: application/json" \
    -d "{
      \"topic\": \"projects/${PROJECT}/topics/${TOPIC}\",
      \"pushConfig\": { \"pushEndpoint\": \"${PUSH_URL}\" },
      \"ackDeadlineSeconds\": 60
    }" > /dev/null || true
}

declare_sub user-events  stats-user-events-sub
declare_sub deck-events  stats-deck-events-sub
declare_sub study-events stats-study-events-sub

echo "Done."
echo "  Topics:        user-events, deck-events, study-events"
echo "  Subscriptions: stats-user-events-sub, stats-deck-events-sub, stats-study-events-sub"
echo "  Push endpoint: ${PUSH_URL}"
