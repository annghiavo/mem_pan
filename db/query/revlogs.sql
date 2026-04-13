-- name: CreateRevlog :one
INSERT INTO revlogs (
  card_id,
  user_id,
  rating,
  duration_ms,
  state,
  elapsed_days,
  stability_before,
  difficulty_before
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: ListRevlogsByCard :many
SELECT * FROM revlogs
WHERE card_id = $1
ORDER BY review_time DESC
LIMIT $2 OFFSET $3;

-- name: ListRevlogsByUser :many
SELECT * FROM revlogs
WHERE user_id = $1
ORDER BY review_time DESC
LIMIT $2 OFFSET $3;
