-- name: GetDeckStats :one
SELECT * FROM deck_stats WHERE deck_id = $1;

-- name: ListDeckStatsByUser :many
SELECT * FROM deck_stats WHERE user_id = $1 ORDER BY updated_at DESC;

-- name: CreateDeckStats :exec
INSERT INTO deck_stats (deck_id, user_id, deck_name)
VALUES ($1, $2, $3)
ON CONFLICT (deck_id) DO NOTHING;

-- name: IncrementDeckTotalCards :exec
UPDATE deck_stats SET
    total_cards = total_cards + 1,
    new_cards   = new_cards + 1,
    updated_at  = CURRENT_TIMESTAMP
WHERE deck_id = $1;

-- name: ShiftDeckCardStates :exec
UPDATE deck_stats SET
    new_cards      = GREATEST(0, new_cards      + $2),
    learning_cards = GREATEST(0, learning_cards + $3),
    review_cards   = GREATEST(0, review_cards   + $4),
    mastered_cards = GREATEST(0, mastered_cards + $5),
    updated_at     = CURRENT_TIMESTAMP
WHERE deck_id = $1;

-- name: UpdateDeckName :exec
UPDATE deck_stats SET
    deck_name  = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE deck_id = $1;

-- name: DeleteDeckStats :exec
DELETE FROM deck_stats WHERE deck_id = $1 AND user_id = $2;
