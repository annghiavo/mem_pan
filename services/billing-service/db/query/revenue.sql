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

-- name: MarkCreatorEarningPaid :one
UPDATE creator_earnings
SET status = 'paid', paid_at = CURRENT_TIMESTAMP
WHERE earning_id = $1
RETURNING *;
