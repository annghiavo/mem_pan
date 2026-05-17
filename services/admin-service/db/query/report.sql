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

-- name: GetReportTarget :one
SELECT target_type, target_id FROM reports
WHERE report_id = $1 LIMIT 1;

-- name: BulkResolveReportsByTarget :many
-- Updates every pending report for the same (target_type, target_id) so that
-- one admin decision covers every duplicate report. Returns all affected rows.
UPDATE reports
SET
    status      = $3,
    resolution  = COALESCE(sqlc.narg('resolution'), resolution),
    admin_note  = COALESCE(sqlc.narg('admin_note'), admin_note),
    resolved_by = sqlc.arg('resolved_by'),
    resolved_at = CURRENT_TIMESTAMP,
    updated_at  = CURRENT_TIMESTAMP
WHERE target_type = $1 AND target_id = $2 AND status = 'pending'
RETURNING *;

-- name: ListReporterIDsByTarget :many
SELECT DISTINCT reporter_id FROM reports
WHERE target_type = $1 AND target_id = $2;

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
