#!/bin/bash
set -e

PROJECT_ID="mempan-cac51"

# Fetch token dynamically from Google Secret Manager instead of hardcoding
echo "Fetching pubsub-push-token from Secret Manager..."
TOKEN=$(gcloud secrets versions access latest --secret="pubsub-push-token" --project="$PROJECT_ID")

if [ -z "$TOKEN" ]; then
  echo "Error: Failed to fetch pubsub-push-token from Secret Manager."
  exit 1
fi

STATS_PUSH="https://stats-service-wzed7v5hbq-eu.a.run.app/internal/pubsub?token=$TOKEN"
NOTIF_PUSH="https://notification-service-wzed7v5hbq-eu.a.run.app/internal/pubsub?token=$TOKEN"
SEARCH_PUSH="https://search-service-wzed7v5hbq-eu.a.run.app/internal/pubsub?token=$TOKEN"
ADMIN_PUSH="https://admin-service-wzed7v5hbq-eu.a.run.app/internal/pubsub?token=$TOKEN"
MODERATION_PUSH="https://moderation-fsrs-service-wzed7v5hbq-eu.a.run.app/internal/pubsub?token=$TOKEN"

create_topic_if_not_exists() {
  local topic=$1
  if gcloud pubsub topics describe "$topic" --project="$PROJECT_ID" &>/dev/null; then
    echo "Topic $topic already exists."
  else
    echo "Creating topic $topic..."
    gcloud pubsub topics create "$topic" --project="$PROJECT_ID"
  fi
}

create_sub_if_not_exists() {
  local topic=$1
  local sub=$2
  local push_url=$3
  
  if gcloud pubsub subscriptions describe "$sub" --project="$PROJECT_ID" &>/dev/null; then
    echo "Subscription $sub already exists."
  else
    echo "Creating subscription $sub for topic $topic..."
    gcloud pubsub subscriptions create "$sub" \
      --topic="$topic" \
      --push-endpoint="$push_url" \
      --ack-deadline=60 \
      --project="$PROJECT_ID"
  fi
}

# 1. Ensure all topics exist
for topic in user-events deck-events study-events report-events moderation-events cron-study-reminder cron-streak-warning card-events; do
  create_topic_if_not_exists "$topic"
done

# 2. Ensure all subscriptions exist
# stats-service
create_sub_if_not_exists "user-events" "stats-user-events-sub" "$STATS_PUSH"
create_sub_if_not_exists "deck-events" "stats-deck-events-sub" "$STATS_PUSH"
create_sub_if_not_exists "study-events" "stats-study-events-sub" "$STATS_PUSH"
create_sub_if_not_exists "card-events" "stats-card-events-sub" "$STATS_PUSH"

# notification-service (using short names matching GCP existing conventions or standardizing)
create_sub_if_not_exists "user-events" "notif-user-events-sub" "$NOTIF_PUSH"
create_sub_if_not_exists "deck-events" "notif-deck-events-sub" "$NOTIF_PUSH"
create_sub_if_not_exists "report-events" "notif-report-events-sub" "$NOTIF_PUSH"
create_sub_if_not_exists "moderation-events" "notif-moderation-events-sub" "$NOTIF_PUSH"
create_sub_if_not_exists "cron-study-reminder" "notif-cron-sub" "$NOTIF_PUSH"
create_sub_if_not_exists "cron-streak-warning" "notif-streak-warning-sub" "$NOTIF_PUSH"

# search-service
create_sub_if_not_exists "user-events" "search-user-events-sub" "$SEARCH_PUSH"
create_sub_if_not_exists "deck-events" "search-deck-events-sub" "$SEARCH_PUSH"
create_sub_if_not_exists "card-events" "search-card-events-sub" "$SEARCH_PUSH"

# admin-service
create_sub_if_not_exists "user-events" "admin-user-events-sub" "$ADMIN_PUSH"
create_sub_if_not_exists "deck-events" "admin-deck-events-sub" "$ADMIN_PUSH"
create_sub_if_not_exists "moderation-events" "admin-moderation-events-sub" "$ADMIN_PUSH"

# moderation-service
create_sub_if_not_exists "deck-events" "moderation-deck-events-sub" "$MODERATION_PUSH"

echo "Production Pub/Sub setup complete."
