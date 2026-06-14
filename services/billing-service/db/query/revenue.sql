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

-- name: UpsertMonthlyRevenuePool :one
INSERT INTO monthly_revenue_pools (
    pool_month,
    gross_amount_vnd,
    creator_pool_amount_vnd,
    platform_amount_vnd,
    status,
    finalized_at
)
VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (pool_month) DO UPDATE
SET gross_amount_vnd = EXCLUDED.gross_amount_vnd,
    creator_pool_amount_vnd = EXCLUDED.creator_pool_amount_vnd,
    platform_amount_vnd = EXCLUDED.platform_amount_vnd,
    status = EXCLUDED.status,
    finalized_at = EXCLUDED.finalized_at
RETURNING *;

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

-- name: UpsertCreatorEarning :one
INSERT INTO creator_earnings (
    pool_month,
    creator_id,
    eligible_learners,
    weighted_score,
    amount_vnd,
    status
)
VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (pool_month, creator_id) DO UPDATE
SET eligible_learners = EXCLUDED.eligible_learners,
    weighted_score = EXCLUDED.weighted_score,
    amount_vnd = EXCLUDED.amount_vnd,
    status = CASE
        WHEN creator_earnings.status = 'paid' THEN creator_earnings.status
        WHEN creator_earnings.status = 'processing' THEN creator_earnings.status
        ELSE EXCLUDED.status
    END
RETURNING *;

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
SET status = $5::text,
    paid_at = CASE WHEN $5::text = 'paid' THEN CURRENT_TIMESTAMP ELSE paid_at END,
    payos_payout_id = $2,
    payos_payout_transaction_id = $3,
    payos_payout_state = $4,
    payout_raw_payload = $6::text::jsonb,
    payout_failed_reason = NULL
WHERE earning_id = $1
RETURNING *;

-- name: MarkCreatorEarningPayoutFailed :one
UPDATE creator_earnings
SET status = 'failed',
    payos_payout_state = $2,
    payout_raw_payload = $3::text::jsonb,
    payout_failed_reason = $4
WHERE earning_id = $1
RETURNING *;
