#!/bin/sh
set -e

EMULATOR="http://${PUBSUB_EMULATOR_HOST:-pubsub-emulator:8085}"
PROJECT="${PUBSUB_PROJECT_ID:-local-dev}"
SECRET="${PUBSUB_PUSH_SECRET:-dev-secret}"
STATS_HOST="${STATS_SERVICE_HOST:-stats-service:8084}"
NOTIFICATION_HOST="${NOTIFICATION_SERVICE_HOST:-notification-service:8085}"
SEARCH_HOST="${SEARCH_SERVICE_HOST:-search-service:8086}"
ADMIN_HOST="${ADMIN_SERVICE_HOST:-admin-service:8083}"
MODERATION_HOST="${MODERATION_SERVICE_HOST:-moderation-service:8087}"

STATS_PUSH_URL="http://${STATS_HOST}/internal/pubsub?token=${SECRET}"
NOTIFICATION_PUSH_URL="http://${NOTIFICATION_HOST}/internal/pubsub?token=${SECRET}"
SEARCH_PUSH_URL="http://${SEARCH_HOST}/internal/pubsub?token=${SECRET}"
ADMIN_PUSH_URL="http://${ADMIN_HOST}/internal/pubsub?token=${SECRET}"
MODERATION_PUSH_URL="http://${MODERATION_HOST}/internal/pubsub?token=${SECRET}"

echo "Waiting for Pub/Sub emulator at ${EMULATOR}..."
until curl -sf -o /dev/null "${EMULATOR}/v1/projects/${PROJECT}/topics"; do
  sleep 2
done

for TOPIC in user-events deck-events study-events report-events moderation-events cron-study-reminder cron-streak-warning; do
  echo "Creating topic ${TOPIC}..."
  curl -s -X PUT "${EMULATOR}/v1/projects/${PROJECT}/topics/${TOPIC}" \
    -H "Content-Type: application/json" \
    -d '{}' > /dev/null || true
done

declare_sub() {
  TOPIC="$1"
  SUB="$2"
  PUSH_URL="$3"
  echo "Creating subscription ${SUB} -> ${PUSH_URL}..."
  curl -s -X PUT "${EMULATOR}/v1/projects/${PROJECT}/subscriptions/${SUB}" \
    -H "Content-Type: application/json" \
    -d "{
      \"topic\": \"projects/${PROJECT}/topics/${TOPIC}\",
      \"pushConfig\": { \"pushEndpoint\": \"${PUSH_URL}\" },
      \"ackDeadlineSeconds\": 60
    }" > /dev/null || true
}

# stats-service subscriptions
declare_sub user-events  stats-user-events-sub  "${STATS_PUSH_URL}"
declare_sub deck-events  stats-deck-events-sub  "${STATS_PUSH_URL}"
declare_sub study-events stats-study-events-sub "${STATS_PUSH_URL}"

# notification-service subscriptions
declare_sub user-events    notification-user-events-sub    "${NOTIFICATION_PUSH_URL}"
declare_sub deck-events    notification-deck-events-sub    "${NOTIFICATION_PUSH_URL}"
declare_sub report-events  notification-report-events-sub  "${NOTIFICATION_PUSH_URL}"
# moderation-events fan-out: notification-service pushes an FCM alert to the
# deck owner when moderation-fsrs-service auto-hides their deck.
declare_sub moderation-events notification-moderation-events-sub "${NOTIFICATION_PUSH_URL}"
# reminder cron jobs — Cloud Scheduler publishes a tick into each topic every
# 15 minutes; notification-service iterates eligible users and dispatches FCM.
declare_sub cron-study-reminder notification-cron-study-reminder-sub "${NOTIFICATION_PUSH_URL}"
declare_sub cron-streak-warning notification-cron-streak-warning-sub "${NOTIFICATION_PUSH_URL}"

# search-service subscriptions
declare_sub user-events  search-user-events-sub  "${SEARCH_PUSH_URL}"
declare_sub deck-events  search-deck-events-sub  "${SEARCH_PUSH_URL}"

# admin-service subscriptions (report.submitted is published to user-events by
# auth-service and to deck-events by deck-service)
declare_sub user-events       admin-user-events-sub        "${ADMIN_PUSH_URL}"
declare_sub deck-events       admin-deck-events-sub        "${ADMIN_PUSH_URL}"
declare_sub moderation-events admin-moderation-events-sub  "${ADMIN_PUSH_URL}"

# moderation-service: subscribes to deck-events to receive card.created /
# card.updated for inline content moderation. On violation it gRPC-calls
# deck-service.AdminUpdateDeckStatus + publishes moderation-events.
declare_sub deck-events  moderation-deck-events-sub  "${MODERATION_PUSH_URL}"

echo "Done."
echo "  Topics:           user-events, deck-events, study-events, report-events, moderation-events,"
echo "                    cron-study-reminder, cron-streak-warning"
echo "  Stats subs:       stats-user-events-sub, stats-deck-events-sub, stats-study-events-sub"
echo "  Notif subs:       notification-user-events-sub, notification-deck-events-sub,"
echo "                    notification-report-events-sub, notification-moderation-events-sub,"
echo "                    notification-cron-study-reminder-sub, notification-cron-streak-warning-sub"
echo "  Search subs:      search-user-events-sub, search-deck-events-sub"
echo "  Admin subs:       admin-user-events-sub, admin-deck-events-sub, admin-moderation-events-sub"
echo "  Moderation subs:  moderation-deck-events-sub"
echo "  Stats push:       ${STATS_PUSH_URL}"
echo "  Notif push:       ${NOTIFICATION_PUSH_URL}"
echo "  Search push:      ${SEARCH_PUSH_URL}"
echo "  Admin push:       ${ADMIN_PUSH_URL}"
echo "  Moderation push:  ${MODERATION_PUSH_URL}"
