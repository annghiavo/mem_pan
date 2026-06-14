#!/bin/bash
set -e

ENV_FILE="services/billing-service/app.env"

if [ ! -f "$ENV_FILE" ]; then
  echo "Error: $ENV_FILE does not exist."
  exit 1
fi

# Load variables from env file
load_var() {
  local var_name=$1
  local value=$(grep -E "^${var_name}=" "$ENV_FILE" | head -n 1 | cut -d'=' -f2-)
  # Strip quotes if present
  value="${value%\"}"
  value="${value#\"}"
  echo "$value"
}

DB_URL=$(load_var "DATABASE_URL")
PAYOS_CLIENT_ID=$(load_var "PAYOS_CLIENT_ID")
PAYOS_API_KEY=$(load_var "PAYOS_API_KEY")
PAYOS_CHECKSUM_KEY=$(load_var "PAYOS_CHECKSUM_KEY")
PAYOS_PAYOUT_CLIENT_ID=$(load_var "PAYOS_PAYOUT_CLIENT_ID")
PAYOS_PAYOUT_API_KEY=$(load_var "PAYOS_PAYOUT_API_KEY")
PAYOS_PAYOUT_CHECKSUM_KEY=$(load_var "PAYOS_PAYOUT_CHECKSUM_KEY")

if [ -z "$DB_URL" ] || [ -z "$PAYOS_CLIENT_ID" ] || [ -z "$PAYOS_API_KEY" ] || [ -z "$PAYOS_CHECKSUM_KEY" ]; then
  echo "Error: Missing required variables in $ENV_FILE."
  exit 1
fi

# Create secrets in Google Secret Manager if they don't exist and add their values
create_secret_if_not_exists() {
  local name=$1
  local value=$2
  
  if gcloud secrets describe "$name" &>/dev/null; then
    echo "Secret $name already exists, adding new version..."
  else
    echo "Creating secret $name..."
    gcloud secrets create "$name" --replication-policy="automatic"
  fi
  
  echo -n "$value" | gcloud secrets versions add "$name" --data-file=-
}

create_secret_if_not_exists "billing-db-url" "$DB_URL"
create_secret_if_not_exists "payos-client-id" "$PAYOS_CLIENT_ID"
create_secret_if_not_exists "payos-api-key" "$PAYOS_API_KEY"
create_secret_if_not_exists "payos-checksum-key" "$PAYOS_CHECKSUM_KEY"
create_secret_if_not_exists "payos-payout-client-id" "$PAYOS_PAYOUT_CLIENT_ID"
create_secret_if_not_exists "payos-payout-api-key" "$PAYOS_PAYOUT_API_KEY"
create_secret_if_not_exists "payos-payout-checksum-key" "$PAYOS_PAYOUT_CHECKSUM_KEY"

echo "All secrets configured successfully."
