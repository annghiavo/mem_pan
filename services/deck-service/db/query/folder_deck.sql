-- name: AddDeckToFolder :one
INSERT INTO folder_decks (folder_id, deck_id)
VALUES ($1, $2)
RETURNING *;

-- name: RemoveDeckFromFolder :exec
DELETE FROM folder_decks WHERE folder_id = $1 AND deck_id = $2;

-- name: ListDecksByFolder :many
SELECT d.*
FROM decks d
JOIN folder_decks fd ON d.deck_id = fd.deck_id
WHERE fd.folder_id = $1 AND d.status != 'deleted'
ORDER BY fd.added_at DESC;

-- name: IsDeckInFolder :one
SELECT EXISTS(
    SELECT 1 FROM folder_decks WHERE folder_id = $1 AND deck_id = $2
) AS exists;
