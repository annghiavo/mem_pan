-- name: CreateCard :one
INSERT INTO cards (user_id, deck_id, note_id, position)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetCardByID :one
SELECT c.card_id, c.user_id, c.deck_id, c.note_id, c.position, c.created_at,
       n.content_front, n.content_back, n.image_url, n.lang_front, n.lang_back
FROM cards c
JOIN notes n ON c.note_id = n.note_id
WHERE c.card_id = $1
LIMIT 1;

-- name: ListCardsByDeck :many
SELECT c.card_id, c.user_id, c.deck_id, c.note_id, c.position, c.created_at,
       n.content_front, n.content_back, n.image_url, n.lang_front, n.lang_back
FROM cards c
JOIN notes n ON c.note_id = n.note_id
WHERE c.deck_id = $1
ORDER BY c.position ASC, c.created_at ASC;

-- name: DeleteCard :exec
DELETE FROM cards WHERE card_id = $1 AND user_id = $2;

-- name: CountCardsByDeck :one
SELECT COUNT(*) FROM cards WHERE deck_id = $1;
