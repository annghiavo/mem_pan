-- name: CreateSubscription :one
INSERT INTO subscriptions (user_id, plan_code, status, current_period_start, current_period_end)
VALUES ($1, $2, 'pending', $3, $4)
RETURNING *;

-- name: GetSubscriptionByID :one
SELECT * FROM subscriptions
WHERE subscription_id = $1
LIMIT 1;

-- name: GetLatestSubscriptionForUser :one
SELECT * FROM subscriptions
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetActiveSubscriptionForUser :one
SELECT * FROM subscriptions
WHERE user_id = $1
  AND status = 'active'
  AND current_period_end > now()
ORDER BY current_period_end DESC
LIMIT 1;

-- name: ActivateSubscription :one
UPDATE subscriptions
SET status = 'active',
    current_period_start = $2,
    current_period_end = $3,
    updated_at = now()
WHERE subscription_id = $1
RETURNING *;

-- name: UpdateSubscriptionStatus :one
UPDATE subscriptions
SET status = $2,
    updated_at = now()
WHERE subscription_id = $1
RETURNING *;

-- name: ExpireSubscriptions :exec
UPDATE subscriptions
SET status = 'expired',
    updated_at = now()
WHERE status IN ('pending', 'active')
  AND current_period_end <= now();

-- name: CreatePaymentTransaction :one
INSERT INTO payment_transactions (
    user_id,
    subscription_id,
    provider,
    provider_payment_id,
    provider_order_code,
    idempotency_key,
    amount_vnd,
    status,
    checkout_url,
    raw_payload
)
VALUES (
    sqlc.arg('user_id'),
    sqlc.arg('subscription_id'),
    'payos',
    sqlc.arg('provider_payment_id'),
    sqlc.arg('provider_order_code'),
    sqlc.arg('idempotency_key'),
    sqlc.arg('amount_vnd'),
    'pending',
    sqlc.arg('checkout_url'),
    sqlc.arg('raw_payload')::jsonb
)
RETURNING *;

-- name: GetPaymentTransactionByOrderCode :one
SELECT * FROM payment_transactions
WHERE provider = 'payos'
  AND provider_order_code = $1
LIMIT 1;

-- name: MarkPaymentTransactionPaid :one
UPDATE payment_transactions
SET status = 'paid',
    paid_at = COALESCE($2, now()),
    raw_payload = COALESCE(sqlc.arg('raw_payload')::jsonb, raw_payload),
    updated_at = now()
WHERE transaction_id = $1
RETURNING *;

-- name: MarkPaymentTransactionStatus :one
UPDATE payment_transactions
SET status = $2,
    raw_payload = COALESCE(sqlc.arg('raw_payload')::jsonb, raw_payload),
    updated_at = now()
WHERE transaction_id = $1
RETURNING *;

-- name: RecordWebhookEvent :one
INSERT INTO payment_webhook_events (provider, event_key, raw_payload)
VALUES (
    'payos',
    sqlc.arg('event_key'),
    sqlc.arg('raw_payload')::jsonb
)
ON CONFLICT (provider, event_key) DO NOTHING
RETURNING *;
