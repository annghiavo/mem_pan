-- name: GetActiveEmailTemplate :one
SELECT * FROM email_templates
WHERE template_key = $1 AND locale = $2 AND is_active = TRUE
LIMIT 1;

-- name: GetEmailTemplate :one
SELECT * FROM email_templates
WHERE template_key = $1 AND locale = $2
LIMIT 1;

-- name: ListEmailTemplates :many
SELECT * FROM email_templates
ORDER BY template_key, locale;

-- name: UpdateEmailTemplate :one
UPDATE email_templates
SET subject    = $3,
    html_body  = $4,
    text_body  = $5,
    updated_by = $6,
    version    = version + 1,
    updated_at = NOW()
WHERE template_key = $1 AND locale = $2
RETURNING *;

-- name: InsertEmailTemplateVersion :exec
INSERT INTO email_template_versions
    (template_id, version, subject, html_body, text_body, updated_by)
VALUES ($1, $2, $3, $4, $5, $6);
