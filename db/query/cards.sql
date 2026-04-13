-- name: CreateCard :one
INSERT INTO cards (
  user_id,
  note_id,
  deck_id,
  state,
  next_review_date
) VALUES (
  $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetCard :one
SELECT * FROM cards
WHERE card_id = $1 LIMIT 1;

-- name: ListDueCardsByDeck :many
SELECT * FROM cards
WHERE deck_id = $1
  AND next_review_date <= NOW()
  AND state != 'new'
ORDER BY next_review_date ASC
LIMIT $2;

-- name: UpdateCardReviewState :one
UPDATE cards
SET state = $2,
    stability = $3,
    difficulty = $4,
    reps = $5,
    lapses = $6,
    scheduled_days = $7,
    next_review_date = $8,
    t_avg = $9,
    last_review_date = CURRENT_TIMESTAMP
WHERE card_id = $1
RETURNING *;
