-- name: InsertRevlog :one
INSERT INTO revlogs (
    user_id, card_id, user_card_id, session_id,
    rating, duration_ms,
    state_before, stability_before, difficulty_before,
    elapsed_days, scheduled_days,
    state_after, stability_after, difficulty_after
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: ListRevlogsByUserCard :many
SELECT * FROM revlogs
WHERE user_card_id = $1
ORDER BY review_time DESC
LIMIT $2;

-- name: ListUsersWithMinReviews :many
-- Users eligible for FSRS weight optimization: those with at least $1 reviews.
-- Driven by the daily optimization cron.
SELECT user_id, COUNT(*) AS review_count
FROM revlogs
GROUP BY user_id
HAVING COUNT(*) >= @min_reviews::bigint
ORDER BY review_count DESC;

-- name: ListReviewLogsForOptimize :many
-- Training samples for the optimizer, oldest first (elapsed_days is relative to
-- the previous review, so chronological order matters).
SELECT card_id, rating, elapsed_days, review_time
FROM revlogs
WHERE user_id = $1
ORDER BY review_time ASC;
