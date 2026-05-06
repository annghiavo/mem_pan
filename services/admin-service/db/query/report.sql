-- name: CreateReport :one
INSERT INTO reports (
    reporter_id,
    target_type,
    target_id,
    reason_category,
    description
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetReport :one
SELECT * FROM reports
WHERE report_id = $1 LIMIT 1;

-- name: ListReports :many
SELECT * FROM reports
WHERE status = COALESCE(sqlc.narg('status_filter'), status)
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountReports :one
SELECT COUNT(*) FROM reports
WHERE status = COALESCE(sqlc.narg('status_filter'), status);

-- name: UpdateReportStatus :one
UPDATE reports
SET
    status = $2,
    resolution = COALESCE(sqlc.narg('resolution'), resolution),
    admin_note = COALESCE(sqlc.narg('admin_note'), admin_note),
    resolved_by = COALESCE(sqlc.narg('resolved_by'), resolved_by),
    resolved_at = CASE WHEN $2::report_status IN ('resolved', 'dismissed') THEN CURRENT_TIMESTAMP ELSE resolved_at END,
    updated_at = CURRENT_TIMESTAMP
WHERE report_id = $1
RETURNING *;

-- name: CreateModerationLog :one
INSERT INTO moderation_logs (
    admin_id,
    action,
    target_type,
    target_id,
    reason,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;
