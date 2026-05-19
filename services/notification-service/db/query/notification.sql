-- name: UpsertFCMToken :one
INSERT INTO fcm_tokens (user_id, token, device_name)
VALUES ($1, $2, $3)
ON CONFLICT (token) DO UPDATE
    SET user_id     = EXCLUDED.user_id,
        device_name = EXCLUDED.device_name,
        updated_at  = NOW()
RETURNING *;

-- name: DeleteFCMToken :exec
DELETE FROM fcm_tokens WHERE token = $1 AND user_id = $2;

-- name: ListFCMTokensByUser :many
SELECT * FROM fcm_tokens WHERE user_id = $1 ORDER BY updated_at DESC;

-- name: LogNotification :exec
INSERT INTO notification_log (user_id, notification_type, channel, recipient, status, error_message)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: CountRecentNotifications :one
-- Used by reminder cron handlers to dedupe within a local day.
SELECT COUNT(*)::bigint
FROM notification_log
WHERE user_id           = $1
  AND notification_type = $2
  AND status            = 'sent'
  AND created_at        >= $3;
