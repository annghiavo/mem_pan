#!/bin/sh
set -e

EMULATOR="http://${PUBSUB_EMULATOR_HOST:-pubsub-emulator:8085}"
PROJECT="${PUBSUB_PROJECT_ID:-local-dev}"
TOPIC="${PUBSUB_TOPIC:-mem-pan-events}"
SUBSCRIPTION="${PUBSUB_SUBSCRIPTION:-stats-push-sub}"
STATS_HOST="${STATS_SERVICE_HOST:-stats-service:8084}"
SECRET="${PUBSUB_PUSH_SECRET:-dev-secret}"
PUSH_ENDPOINT="http://${STATS_HOST}/internal/pubsub?token=${SECRET}"

echo "Waiting for Pub/Sub emulator at ${EMULATOR}..."
until curl -sf -o /dev/null "${EMULATOR}/v1/projects/${PROJECT}/topics"; do
  sleep 2
done

echo "Creating topic ${TOPIC}..."
curl -sf -X PUT "${EMULATOR}/v1/projects/${PROJECT}/topics/${TOPIC}" \
  -H "Content-Type: application/json" \
  -d '{}' > /dev/null

echo "Creating push subscription ${SUBSCRIPTION} -> ${PUSH_ENDPOINT}..."
curl -sf -X PUT "${EMULATOR}/v1/projects/${PROJECT}/subscriptions/${SUBSCRIPTION}" \
  -H "Content-Type: application/json" \
  -d "{
    \"topic\": \"projects/${PROJECT}/topics/${TOPIC}\",
    \"pushConfig\": { \"pushEndpoint\": \"${PUSH_ENDPOINT}\" },
    \"ackDeadlineSeconds\": 30
  }" > /dev/null

echo "Done."
echo "  Topic:        projects/${PROJECT}/topics/${TOPIC}"
echo "  Subscription: projects/${PROJECT}/subscriptions/${SUBSCRIPTION}"
echo "  Push endpoint: ${PUSH_ENDPOINT}"
