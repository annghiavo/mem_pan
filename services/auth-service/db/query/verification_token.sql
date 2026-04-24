-- name: CreateVerificationToken :one
INSERT INTO verification_tokens (user_id, token_hash, type, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetVerificationTokenByHash :one
SELECT * FROM verification_tokens WHERE token_hash = $1 LIMIT 1;

-- name: MarkVerificationTokenUsed :exec
UPDATE verification_tokens
SET used_at = now()
WHERE token_hash = $1;

-- name: DeleteExpiredVerificationTokens :exec
DELETE FROM verification_tokens WHERE user_id = $1 AND expires_at < now();
