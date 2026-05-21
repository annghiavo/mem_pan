#!/usr/bin/env bash
# Wipes all data from the 6 Postgres databases and the Elasticsearch indices,
# preserving every table/index structure. The notification-service email
# template tables (email_templates, email_template_versions) are kept intact.
#
# Tables are discovered at runtime from information_schema, so the script
# tolerates schema drift (missing migrations, renamed/dropped tables).
#
# Usage: scripts/reset-data.sh [-y]
#   -y    skip the confirmation prompt
#
# Requirements: docker (used to run psql) and curl.

set -euo pipefail

ASSUME_YES=0
if [[ "${1:-}" == "-y" ]]; then
  ASSUME_YES=1
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICES_DIR="$REPO_ROOT/services"

read_env() {
  # read_env <key> <env-file>
  grep -E "^$1=" "$2" | head -n1 | cut -d= -f2-
}

# truncate_all <database-url> [preserve-table ...]
# Truncates every table in the public schema except the preserve list.
truncate_all() {
  local url="$1"; shift
  local preserve_csv=""
  for t in "$@"; do
    preserve_csv+="'${t}',"
  done
  preserve_csv="${preserve_csv%,}"
  [[ -z "$preserve_csv" ]] && preserve_csv="''"

  local sql
  sql=$(cat <<SQL
DO \$\$
DECLARE
  tbls text;
BEGIN
  SELECT string_agg(format('%I.%I', schemaname, tablename), ', ')
    INTO tbls
  FROM pg_tables
  WHERE schemaname = 'public'
    AND tablename NOT IN ($preserve_csv);
  IF tbls IS NOT NULL THEN
    EXECUTE 'TRUNCATE TABLE ' || tbls || ' RESTART IDENTITY CASCADE';
  END IF;
END
\$\$;
SQL
)

  docker run --rm -i postgres:16 \
    psql "$url" -v ON_ERROR_STOP=1 -c "$sql" >/dev/null
}

if [[ "$ASSUME_YES" -ne 1 ]]; then
  echo "This will DELETE ALL DATA in:"
  echo "  - 6 Postgres databases (auth, deck, study, admin, stats, notification)"
  echo "  - 4 Elasticsearch indices (decks, folders, cards, users)"
  echo "Email templates in notification-service will be preserved."
  read -r -p "Type 'yes' to continue: " confirm
  [[ "$confirm" == "yes" ]] || { echo "Aborted."; exit 1; }
fi

echo "==> auth-service"
truncate_all "$(read_env DATABASE_URL "$SERVICES_DIR/auth-service/app.env")"

echo "==> deck-service"
truncate_all "$(read_env DATABASE_URL "$SERVICES_DIR/deck-service/app.env")"

echo "==> study-service"
truncate_all "$(read_env DATABASE_URL "$SERVICES_DIR/study-service/app.env")"

echo "==> admin-service"
truncate_all "$(read_env DATABASE_URL "$SERVICES_DIR/admin-service/app.env")"

echo "==> stats-service"
truncate_all "$(read_env DATABASE_URL "$SERVICES_DIR/stats-service/app.env")"

echo "==> notification-service (keeping email_templates, email_template_versions)"
truncate_all "$(read_env DATABASE_URL "$SERVICES_DIR/notification-service/app.env")" \
  email_templates email_template_versions

echo "==> Elasticsearch (search-service)"
ES_ENV="$SERVICES_DIR/search-service/app.env"
ES_URL=$(read_env ELASTICSEARCH_URL "$ES_ENV")
ES_KEY=$(read_env ELASTICSEARCH_API_KEY "$ES_ENV")
for key in ES_DECK_INDEX ES_FOLDER_INDEX ES_CARD_INDEX ES_USER_INDEX; do
  index=$(read_env "$key" "$ES_ENV")
  echo "    $index"
  curl -fsS -X POST "$ES_URL/$index/_delete_by_query?refresh=true&conflicts=proceed" \
    -H "Authorization: ApiKey $ES_KEY" \
    -H "Content-Type: application/json" \
    -d '{"query":{"match_all":{}}}' >/dev/null
done

echo "Done."
