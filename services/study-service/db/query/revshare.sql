-- name: UpsertStudySessionMetrics :one
INSERT INTO study_session_metrics (
    session_id,
    user_id,
    deck_id,
    creator_id,
    card_views,
    reviewed_cards,
    total_active_ms,
    weighted_score,
    is_revshare_eligible,
    invalid_reason
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (session_id) DO UPDATE
SET creator_id = EXCLUDED.creator_id,
    card_views = EXCLUDED.card_views,
    reviewed_cards = EXCLUDED.reviewed_cards,
    total_active_ms = EXCLUDED.total_active_ms,
    weighted_score = EXCLUDED.weighted_score,
    is_revshare_eligible = EXCLUDED.is_revshare_eligible,
    invalid_reason = EXCLUDED.invalid_reason
RETURNING *;

-- name: UpsertMonthlyRevenuePool :one
INSERT INTO monthly_revenue_pools (
    pool_month,
    gross_amount_vnd,
    creator_pool_amount_vnd,
    platform_amount_vnd,
    status
)
VALUES ($1, $2, $3, $2 - $3, 'draft')
ON CONFLICT (pool_month) DO UPDATE
SET gross_amount_vnd = EXCLUDED.gross_amount_vnd,
    creator_pool_amount_vnd = EXCLUDED.creator_pool_amount_vnd,
    platform_amount_vnd = EXCLUDED.platform_amount_vnd,
    status = 'draft',
    finalized_at = NULL
RETURNING *;

-- name: DeleteCreatorEarningsForMonth :exec
DELETE FROM creator_earnings WHERE pool_month = $1;

-- name: InsertCreatorEarningsForMonth :many
WITH creator_scores AS (
    SELECT
        creator_id,
        COUNT(DISTINCT user_id)::integer AS eligible_learners,
        SUM(weighted_score)::numeric(14,4) AS weighted_score
    FROM study_session_metrics
    WHERE is_revshare_eligible = TRUE
      AND creator_id IS NOT NULL
      AND created_at >= sqlc.arg('month_start')::timestamptz
      AND created_at < sqlc.arg('month_end')::timestamptz
    GROUP BY creator_id
    HAVING COUNT(DISTINCT user_id) >= sqlc.arg('min_learners')::integer
),
totals AS (
    SELECT COALESCE(SUM(weighted_score), 0)::numeric(14,4) AS total_score
    FROM creator_scores
),
allocations AS (
    SELECT
        sqlc.arg('pool_month')::date AS pool_month,
        creator_scores.creator_id,
        creator_scores.eligible_learners,
        creator_scores.weighted_score,
        CASE
            WHEN totals.total_score <= 0 THEN 0
            ELSE LEAST(
                ROUND(sqlc.arg('creator_pool_amount_vnd')::numeric * creator_scores.weighted_score / totals.total_score)::bigint,
                sqlc.arg('creator_cap_amount_vnd')::bigint
            )
        END AS amount_vnd
    FROM creator_scores, totals
)
INSERT INTO creator_earnings (
    pool_month,
    creator_id,
    eligible_learners,
    weighted_score,
    amount_vnd
)
SELECT pool_month, creator_id, eligible_learners, weighted_score, amount_vnd
FROM allocations
WHERE amount_vnd > 0
RETURNING *;

-- name: FinalizeRevenuePool :one
UPDATE monthly_revenue_pools
SET status = 'finalized',
    finalized_at = now()
WHERE pool_month = $1
RETURNING *;

-- name: ListCreatorEarningsByMonth :many
SELECT * FROM creator_earnings
WHERE pool_month = $1
ORDER BY amount_vnd DESC, creator_id;
