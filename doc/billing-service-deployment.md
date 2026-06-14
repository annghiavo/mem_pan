# Billing Service Deployment Notes

Status date: 2026-06-11

This document records the current deployment/publishing state of `billing-service`,
the billing routes exposed by the service, and the remaining work for PayOS
automatic creator payouts.

## Summary

| Area | Current state | Notes |
| --- | --- | --- |
| Docker image | Configured | `services/billing-service/Dockerfile` builds the Go server and exposes HTTP `8088` and gRPC `9098`. |
| Local compose service | Configured | `deploy/docker-compose.yml` defines `billing-service` and injects `AUTH_SERVICE_ADDRESS=auth-service:9090`. |
| Internal consumers | Configured | `deck-service` and `study-service` both use `BILLING_SERVICE_ADDRESS=billing-service:9098`. |
| Local public billing API | Published | Traefik routes `/v1/billing`, `/v1/admin/revenue`, and `/v1/creators` to billing-service. |
| Local admin/creator billing routes | Published | `/v1/admin/revenue` uses higher Traefik priority than the generic admin-service `/v1/admin` route. |
| Production API Gateway | Published in spec | `deploy/api-gateway/openapi.yaml` includes billing, admin revenue, and creator earnings routes. Deploy a new API Gateway config to apply it. |
| Cloud Run service manifest | Missing | There is no `services/billing-service/cloudrun.yaml`. Existing Cloud Run YAML files for several services are placeholders. |
| PayOS collection | Implemented | Checkout link creation and webhook processing exist for Plus subscriptions. |
| PayOS automatic payout | Implemented | Billing-service can create single and batch PayOS payouts and check payout-account balance. |

## Docker And Local Compose

Billing service is included in local Docker Compose:

```yaml
billing-service:
  build:
    context: ..
    dockerfile: services/billing-service/Dockerfile
  env_file:
    - ../services/billing-service/app.env
  environment:
    - AUTH_SERVICE_ADDRESS=auth-service:9090
  expose:
    - "8088"
    - "9098"
```

The Dockerfile builds `./cmd/server`, copies it into an Alpine runtime image,
and exposes:

- `8088`: HTTP API and h2c gateway.
- `9098`: standalone gRPC server.

Internal billing clients are wired in compose:

- `deck-service`: `BILLING_SERVICE_ADDRESS=billing-service:9098`
- `study-service`: `BILLING_SERVICE_ADDRESS=billing-service:9098`

## Published HTTP Routes

Billing-service registers these HTTP handlers:

| Route | Purpose | Current gateway state |
| --- | --- | --- |
| `POST /v1/billing/checkout` | Create PayOS checkout link for Plus subscription | Published locally by Traefik |
| `GET /v1/billing/banks` | Return cached VietQR bank list for payout account selection | Published locally by Traefik |
| `GET /v1/billing/subscription/me` | Get current user's Plus subscription status | Published locally by Traefik |
| `POST /v1/billing/webhooks/payos` | Receive PayOS payment webhook | Published locally by Traefik |
| `GET /v1/admin/revenue/pools` | List monthly revenue pools | Published locally by Traefik |
| `GET /v1/admin/revenue/payouts?month=YYYY-MM-DD` | List creator earnings for a month | Published locally by Traefik |
| `POST /v1/admin/revenue/payouts/pay` | Create a single PayOS creator payout | Published locally by Traefik |
| `POST /v1/admin/revenue/payouts/batch` | Create a batch PayOS creator payout | Published locally by Traefik |
| `GET /v1/admin/revenue/payouts/balance` | Check PayOS payout-account balance | Published locally by Traefik |
| `POST /v1/admin/revenue/payouts/mark-paid` | Backward-compatible alias that now creates a PayOS payout | Published locally by Traefik |
| `GET /v1/creators/me/earnings` | Creator views own earnings | Published locally by Traefik |
| `GET /v1/creators/me/payout-account` | Creator views saved payout destination | Published locally by Traefik |
| `PUT /v1/creators/me/payout-account` | Creator saves payout destination from selected bank details | Published locally by Traefik |
| `POST /v1/creators/me/withdrawals` | Creator requests payout for their own eligible earning | Published locally by Traefik |
| `GET /healthz` | Health check | Direct service only unless a gateway route is added |

Local Traefik currently has:

```yaml
traefik.http.routers.billing.rule=PathPrefix(`/v1/billing`)
traefik.http.routers.billing-admin-revenue.rule=PathPrefix(`/v1/admin/revenue`)
traefik.http.routers.billing-creators.rule=PathPrefix(`/v1/creators`)
```

`billing-admin-revenue` and `billing-creators` use higher priority than generic
service routers so the billing-owned paths are not swallowed by admin-service.

## gRPC Surface

The billing gRPC API currently exposes only internal subscription checks:

```proto
service BillingService {
  rpc CheckPlusAccess(CheckPlusAccessRequest) returns (CheckPlusAccessResponse);
  rpc ExpireSubscriptions(ExpireSubscriptionsRequest) returns (ExpireSubscriptionsResponse);
}
```

There is no gRPC payout API.

## Configuration

Required runtime configuration:

| Env var | Required | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | Yes | Billing database connection. `DB_URL` and `DIRECT_URL` are fallback inputs. |
| `AUTH_SERVICE_ADDRESS` | Yes | gRPC address for token verification. Defaults to `localhost:9090`. |
| `PAYOS_CLIENT_ID` | Yes | PayOS API credential. |
| `PAYOS_API_KEY` | Yes | PayOS API credential. |
| `PAYOS_CHECKSUM_KEY` | Yes | Used for payment-link and webhook signatures. |
| `PAYOS_BASE_URL` | No | Defaults to `https://api-merchant.payos.vn`. |
| `PAYOS_PAYOUT_CLIENT_ID` | Yes | PayOS disbursement-channel client ID for Chi/payout APIs. |
| `PAYOS_PAYOUT_API_KEY` | Yes | PayOS disbursement-channel API key for Chi/payout APIs. |
| `PAYOS_PAYOUT_CHECKSUM_KEY` | Yes | PayOS disbursement-channel checksum key for payout signatures. |
| `PAYOS_RETURN_URL` | No | Defaults to `${APP_BASE_URL}/billing/return`. |
| `PAYOS_CANCEL_URL` | No | Defaults to `${APP_BASE_URL}/billing/cancel`. |
| `PLUS_MONTHLY_AMOUNT_VND` | No | Defaults to `49000`. |
| `PLUS_YEARLY_AMOUNT_VND` | No | Defaults to `490000`. |

## Database State

Billing migrations currently create:

- `subscriptions`
- `payment_transactions`
- `payment_webhook_events`
- `monthly_revenue_pools`
- `creator_earnings`
- `creator_payout_accounts`

The revenue-share schema stores monthly earning amounts and PayOS payout state:

```sql
creator_earnings (
  earning_id UUID PRIMARY KEY,
  pool_month DATE NOT NULL,
  creator_id UUID NOT NULL,
  amount_vnd BIGINT NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  paid_at TIMESTAMPTZ,
  payout_reference_id TEXT,
  payout_idempotency_key TEXT,
  payout_to_bin TEXT,
  payout_to_account_number TEXT,
  payout_to_account_name TEXT,
  payos_payout_id TEXT,
  payos_payout_transaction_id TEXT,
  payos_payout_state TEXT,
  payout_raw_payload JSONB,
  payout_requested_at TIMESTAMPTZ,
  payout_failed_reason TEXT
)
```

```sql
creator_payout_accounts (
  creator_id UUID PRIMARY KEY,
  bank_bin TEXT NOT NULL,
  bank_code TEXT NOT NULL,
  bank_short_name TEXT NOT NULL,
  bank_name TEXT NOT NULL,
  bank_logo TEXT,
  account_number TEXT NOT NULL,
  account_name TEXT NOT NULL,
  verified_at TIMESTAMPTZ
)
```

## PayOS Collection Flow

Implemented flow:

1. User calls `POST /v1/billing/checkout`.
2. Billing-service creates a pending subscription.
3. Billing-service calls PayOS `POST /v2/payment-requests`.
4. PayOS returns `checkoutUrl`, `paymentLinkId`, QR data, and status.
5. Billing-service stores a `payment_transactions` row.
6. PayOS sends webhook to `POST /v1/billing/webhooks/payos`.
7. Billing-service verifies webhook signature and amount.
8. Billing-service marks the payment as paid and activates the subscription.

The PayOS client implements payment-link creation and payout creation.

## Current Creator Payout Flow

Implemented flow:

1. Admin gets revenue pools with `GET /v1/admin/revenue/pools`.
2. Admin gets creator earnings with `GET /v1/admin/revenue/payouts?month=YYYY-MM-DD`.
3. Admin calls `POST /v1/admin/revenue/payouts/pay` with `earning_id`, `to_bin`,
   and `to_account_number`.
4. Billing-service validates `amount_vnd > 100000`, records payout metadata,
   and calls PayOS `POST /v1/payouts` with idempotency and signature headers.
5. Billing-service stores PayOS payout IDs, transaction state, and raw response.
6. If PayOS returns a completed/success state, the earning becomes `paid`;
   otherwise it remains `processing`.

Batch payout is available at `POST /v1/admin/revenue/payouts/batch`. Payout
balance is available at `GET /v1/admin/revenue/payouts/balance`.

Creators can view their own earning rows with `GET /v1/creators/me/earnings`
and save their payout destination with `PUT /v1/creators/me/payout-account`.
The saved account stores the VietQR/PayOS bank identifiers (`bank_bin`,
`bank_code`, `bank_short_name`, `bank_name`, optional `bank_logo`) plus the
account number and account name. The frontend should load bank options from
`GET /v1/billing/banks`; billing-service fetches VietQR server-side and caches
transfer-supported banks for 24 hours.

Creators request a PayOS payout for one eligible row with
`POST /v1/creators/me/withdrawals`. Billing-service checks that the selected
earning belongs to the authenticated creator. If the withdrawal request does not
include destination fields, billing-service uses the saved payout account.
The PayOS payout request itself uses `toBin` and `toAccountNumber`; bank name,
short name, and logo are stored only for product display/audit metadata.

## PayOS Automatic Payout Requirement

PayOS payout APIs are documented under the "Chi" section and are separate from
payment collection.

Relevant PayOS endpoints:

- `POST /v1/payouts`: create one payout.
- `GET /v1/payouts`: list payouts.
- `POST /v1/payouts/batch`: create a payout batch.
- `GET /v1/payouts/{payoutId}`: get payout details.
- `POST /v1/payouts/estimate-credit`: estimate payout cost.
- `GET /v1/payouts-account/balance`: get payout account balance.

Payout requests require separate PayOS disbursement-channel credentials. They
use the same header names as payment APIs plus payout-specific request headers:

- `x-client-id`
- `x-api-key`
- `x-idempotency-key`
- `x-signature`

Do not reuse `PAYOS_CLIENT_ID`, `PAYOS_API_KEY`, or `PAYOS_CHECKSUM_KEY` from the
payment channel for payout requests. Use `PAYOS_PAYOUT_CLIENT_ID`,
`PAYOS_PAYOUT_API_KEY`, and `PAYOS_PAYOUT_CHECKSUM_KEY`.

Product withdrawal rule:

- A creator payout must be rejected unless `amount_vnd > 100000`.
- The UI and backend should both show this as "minimum withdrawal amount is over
  100,000 VND".
- Backend validation is authoritative.

## Remaining Work

Automatic payout creation is implemented, but production hardening still needs:

1. Creator payout destination verification.
   - Billing-service stores creator payout destinations in
     `creator_payout_accounts` and withdrawals can use the saved account.
   - Add bank-account holder verification before marking `verified_at`.

2. Payout reconciliation.
   - If PayOS supports payout callbacks, handle them idempotently.
   - Otherwise poll `GET /v1/payouts/{payoutId}` and reconcile `processing`
     payouts.

3. Broader payout client coverage.
   - `GET /v1/payouts`
   - `GET /v1/payouts/{payoutId}`
   - `POST /v1/payouts/estimate-credit`

4. Integration tests with PayOS test credentials or mocked HTTP transport.

## Publishing Work Still Needed

Production still requires deployment operations:

1. Add billing-service deployment commands or a Cloud Run manifest.
2. Add required PayOS and database secrets in Secret Manager.
3. Deploy a new API Gateway config from `deploy/api-gateway/openapi.yaml`.
4. Run smoke tests against the public gateway.

## Post-Deployment Smoke Tests

Local Traefik:

```bash
curl -sS http://localhost:8000/v1/billing/subscription/me \
  -H "Authorization: Bearer $TOKEN"
```

Direct local service:

```bash
curl -sS http://localhost:8088/healthz
```

Expected checks:

- `POST /v1/billing/checkout` returns a PayOS `checkout_url`.
- PayOS webhook with invalid signature is rejected.
- Duplicate PayOS webhook is idempotent.
- Valid PayOS webhook activates subscription.
- `deck-service` and `study-service` can call `CheckPlusAccess`.
- Admin revenue routes are reachable through the chosen gateway path.
- Creator earnings route is reachable through the chosen gateway path.
- `POST /v1/admin/revenue/payouts/pay` rejects `amount_vnd <= 100000`.
- `GET /v1/admin/revenue/payouts/balance` reaches the PayOS payout account.
- `POST /v1/admin/revenue/payouts/pay` creates a PayOS payout with payout-channel credentials.
- `POST /v1/admin/revenue/payouts/batch` creates PayOS payouts for eligible rows and reports per-item failures.
