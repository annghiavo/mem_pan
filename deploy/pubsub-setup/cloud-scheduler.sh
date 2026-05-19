#!/bin/bash
# Provisions the two reminder cron jobs in Google Cloud Scheduler.
#
# Each cron publishes a tick message into a Pub/Sub topic; notification-service
# is the only push subscriber and iterates eligible users on each tick.
#
# Why every 15 minutes?
#   The cron handler matches users whose (optimal_hour - 15min) OR
#   reminder_local_time (default 21:00) falls inside the window
#   (now - 15m, now]. Firing more often than the window length would
#   double-send the same user; firing less often would skip users near the
#   window boundary. 15-minute granularity is the natural choice.
#
# Requirements:
#   gcloud auth login
#   gcloud config set project <project-id>
#   Topics cron-study-reminder and cron-streak-warning must exist (created by
#   terraform or `gcloud pubsub topics create`).
#
# Usage:
#   PROJECT=mempan-prod LOCATION=asia-southeast1 ./cloud-scheduler.sh

set -euo pipefail

PROJECT="${PROJECT:?PROJECT environment variable required}"
LOCATION="${LOCATION:-asia-southeast1}"

STUDY_TOPIC="cron-study-reminder"
STREAK_TOPIC="cron-streak-warning"

# Schedules are in UTC. Every 15 minutes covers all timezones — the handler
# filters per-user using IANA tz data.
SCHEDULE="*/15 * * * *"

create_job() {
  local NAME="$1"
  local TOPIC="$2"
  local EVENT_TYPE="$3"
  local DESCRIPTION="$4"

  # The notification-service push handler expects this envelope shape:
  #   { "event_type": "<type>", "data": "<base64({})>" }
  # JSON marshals []byte as base64, so we pre-encode {} → "e30=".
  local BODY='{"event_type":"'"${EVENT_TYPE}"'","data":"e30="}'

  echo "Provisioning Cloud Scheduler job ${NAME} → projects/${PROJECT}/topics/${TOPIC}"

  # Use `describe` to detect whether the job already exists, and choose
  # create vs update accordingly.
  if gcloud scheduler jobs describe "${NAME}" \
        --location="${LOCATION}" --project="${PROJECT}" >/dev/null 2>&1; then
    gcloud scheduler jobs update pubsub "${NAME}" \
      --location="${LOCATION}" \
      --project="${PROJECT}" \
      --schedule="${SCHEDULE}" \
      --time-zone="UTC" \
      --topic="${TOPIC}" \
      --message-body="${BODY}" \
      --description="${DESCRIPTION}"
  else
    gcloud scheduler jobs create pubsub "${NAME}" \
      --location="${LOCATION}" \
      --project="${PROJECT}" \
      --schedule="${SCHEDULE}" \
      --time-zone="UTC" \
      --topic="${TOPIC}" \
      --message-body="${BODY}" \
      --description="${DESCRIPTION}"
  fi
}

create_job "cron-study-reminder" "${STUDY_TOPIC}" "cron.study_reminder" \
  "Tick every 15 min — notification-service sends study reminder 15 min before optimal_hour."

create_job "cron-streak-warning" "${STREAK_TOPIC}" "cron.streak_warning" \
  "Tick every 15 min — notification-service warns users about losing their streak at their local reminder time."

echo "Done. Two scheduler jobs (every 15 min UTC) are publishing into ${STUDY_TOPIC} and ${STREAK_TOPIC}."
