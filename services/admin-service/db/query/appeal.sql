-- name: CreateDeckAppeal :one
INSERT INTO deck_appeals (
    token,
    deck_id,
    user_id,
    deck_name,
    moderation_reason
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetDeckAppealByID :one
SELECT * FROM deck_appeals
WHERE appeal_id = $1
LIMIT 1;

-- name: GetDeckAppealByToken :one
SELECT * FROM deck_appeals
WHERE token = $1
LIMIT 1;

-- name: GetDeckAppealByDeck :one
SELECT * FROM deck_appeals
WHERE deck_id = $1
LIMIT 1;

-- name: ListDeckAppeals :many
SELECT * FROM deck_appeals
WHERE status = COALESCE(sqlc.narg('status_filter'), status)
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountDeckAppeals :one
SELECT COUNT(*) FROM deck_appeals
WHERE status = COALESCE(sqlc.narg('status_filter'), status);

-- name: SubmitDeckAppeal :one
UPDATE deck_appeals
SET status       = 'submitted',
    user_message = sqlc.arg('user_message'),
    submitted_at = CURRENT_TIMESTAMP,
    updated_at   = CURRENT_TIMESTAMP
WHERE token = $1
  AND status = 'pending'
RETURNING *;

-- name: DecideDeckAppeal :one
UPDATE deck_appeals
SET status        = sqlc.arg('status'),
    decided_by    = sqlc.arg('decided_by'),
    decision_note = sqlc.narg('decision_note'),
    decided_at    = CURRENT_TIMESTAMP,
    updated_at    = CURRENT_TIMESTAMP
WHERE appeal_id = $1
  AND status = 'submitted'
RETURNING *;
