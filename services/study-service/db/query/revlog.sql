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
