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

# --- FSRS weight optimization (daily) ---------------------------------------
# Unlike the reminder ticks, this is an HTTP job hitting study-service directly
# (study-service has no Pub/Sub push subscriber). study-service iterates users
# with >= FSRS_OPTIMIZE_MIN_REVIEWS reviews and re-tunes their 21 FSRS weights
# via moderation-fsrs-service. The endpoint can run for minutes, so we widen the
# attempt deadline. Auth: Cloud Run run.invoker via OIDC + a shared X-Cron-Secret.
#
# Requires (in addition to PROJECT/LOCATION):
#   STUDY_SERVICE_URL   e.g. https://study-service-xxxx.asia-southeast1.run.app
#   CRON_SECRET         same value as study-service's CRON_SECRET env
#   SCHEDULER_SA        service account email with roles/run.invoker on study-service
FSRS_SCHEDULE="${FSRS_SCHEDULE:-0 18 * * *}"   # 18:00 UTC daily (~01:00 Asia/Bangkok, off-peak)

create_fsrs_optimize_job() {
  if [[ -z "${STUDY_SERVICE_URL:-}" || -z "${CRON_SECRET:-}" || -z "${SCHEDULER_SA:-}" ]]; then
    echo "Skipping cron-fsrs-optimize: set STUDY_SERVICE_URL, CRON_SECRET and SCHEDULER_SA to provision it."
    return 0
  fi

  local NAME="cron-fsrs-optimize"
  local URI="${STUDY_SERVICE_URL%/}/internal/fsrs/optimize"
  echo "Provisioning Cloud Scheduler job ${NAME} → ${URI}"

  local -a ARGS=(
    --location="${LOCATION}"
    --project="${PROJECT}"
    --schedule="${FSRS_SCHEDULE}"
    --time-zone="UTC"
    --uri="${URI}"
    --http-method=POST
    --headers="X-Cron-Secret=${CRON_SECRET}"
    --oidc-service-account-email="${SCHEDULER_SA}"
    --oidc-token-audience="${STUDY_SERVICE_URL%/}"
    --attempt-deadline=1800s
    --description="Daily — study-service re-tunes FSRS weights for users with enough review history."
  )

  if gcloud scheduler jobs describe "${NAME}" \
        --location="${LOCATION}" --project="${PROJECT}" >/dev/null 2>&1; then
    gcloud scheduler jobs update http "${NAME}" "${ARGS[@]}"
  else
    gcloud scheduler jobs create http "${NAME}" "${ARGS[@]}"
  fi
}

create_fsrs_optimize_job

echo "Done. Reminder jobs publish into ${STUDY_TOPIC} / ${STREAK_TOPIC}; cron-fsrs-optimize (daily) hits study-service if configured."
