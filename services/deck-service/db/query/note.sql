-- name: CreateNote :one
INSERT INTO notes (user_id, content_front, content_back, image_url, lang_front, lang_back)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetNoteByID :one
SELECT * FROM notes WHERE note_id = $1 LIMIT 1;

-- name: UpdateNote :one
UPDATE notes
SET content_front = COALESCE(sqlc.narg('content_front'), content_front),
    content_back  = COALESCE(sqlc.narg('content_back'), content_back),
    image_url     = COALESCE(sqlc.narg('image_url'), image_url),
    lang_front    = COALESCE(sqlc.narg('lang_front'), lang_front),
    lang_back     = COALESCE(sqlc.narg('lang_back'), lang_back),
    updated_at    = now()
WHERE note_id = sqlc.arg('note_id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- name: DeleteNote :exec
DELETE FROM notes WHERE note_id = $1 AND user_id = $2;
