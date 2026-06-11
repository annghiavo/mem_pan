-- name: CreateMonthlyRevenuePool :one
INSERT INTO monthly_revenue_pools (
    pool_month,
    gross_amount_vnd,
    creator_pool_amount_vnd,
    platform_amount_vnd,
    status
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetMonthlyRevenuePools :many
SELECT * FROM monthly_revenue_pools
ORDER BY pool_month DESC;

-- name: CreateCreatorEarning :one
INSERT INTO creator_earnings (
    pool_month,
    creator_id,
    eligible_learners,
    weighted_score,
    amount_vnd,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetCreatorEarningsByMonth :many
SELECT * FROM creator_earnings
WHERE pool_month = $1
ORDER BY amount_vnd DESC;

-- name: GetMyEarnings :many
SELECT * FROM creator_earnings
WHERE creator_id = $1
ORDER BY pool_month DESC;

-- name: GetCreatorEarningByID :one
SELECT * FROM creator_earnings
WHERE earning_id = $1
LIMIT 1;

-- name: MarkCreatorEarningPayoutProcessing :one
UPDATE creator_earnings
SET status = 'processing',
    payout_reference_id = $2,
    payout_idempotency_key = $3,
    payout_to_bin = $4,
    payout_to_account_number = $5,
    payout_to_account_name = $6,
    payout_requested_at = CURRENT_TIMESTAMP,
    payout_failed_reason = NULL
WHERE earning_id = $1
  AND status IN ('pending', 'failed')
RETURNING *;

-- name: MarkCreatorEarningPayoutPaid :one
UPDATE creator_earnings
SET status = $5,
    paid_at = CASE WHEN $5 = 'paid' THEN CURRENT_TIMESTAMP ELSE paid_at END,
    payos_payout_id = $2,
    payos_payout_transaction_id = $3,
    payos_payout_state = $4,
    payout_raw_payload = $6,
    payout_failed_reason = NULL
WHERE earning_id = $1
RETURNING *;

-- name: MarkCreatorEarningPayoutFailed :one
UPDATE creator_earnings
SET status = 'failed',
    payos_payout_state = $2,
    payout_raw_payload = $3,
    payout_failed_reason = $4
WHERE earning_id = $1
RETURNING *;
