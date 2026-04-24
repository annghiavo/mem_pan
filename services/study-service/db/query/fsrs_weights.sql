-- name: GetActiveWeights :one
SELECT * FROM user_fsrs_weights
WHERE user_id = $1 AND is_active = TRUE
LIMIT 1;

-- name: DeactivateWeights :exec
UPDATE user_fsrs_weights SET is_active = FALSE
WHERE user_id = $1 AND is_active = TRUE;

-- name: GetNextWeightVersion :one
SELECT COALESCE(MAX(version), 0) + 1 AS next_version
FROM user_fsrs_weights
WHERE user_id = $1;

-- name: InsertWeights :one
INSERT INTO user_fsrs_weights (user_id, version, weights, is_active, trained_on_reviews, training_loss)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
