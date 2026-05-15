CREATE TABLE email_templates (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    template_key  TEXT        NOT NULL,
    locale        TEXT        NOT NULL DEFAULT 'en',
    subject       TEXT        NOT NULL,
    html_body     TEXT        NOT NULL,
    text_body     TEXT        NOT NULL,
    variables     JSONB       NOT NULL DEFAULT '[]'::jsonb,
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    version       INT         NOT NULL DEFAULT 1,
    updated_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT email_templates_key_locale_unique UNIQUE (template_key, locale)
);
CREATE INDEX idx_email_templates_key ON email_templates(template_key);

CREATE TABLE email_template_versions (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id  UUID        NOT NULL REFERENCES email_templates(id) ON DELETE CASCADE,
    version      INT         NOT NULL,
    subject      TEXT        NOT NULL,
    html_body    TEXT        NOT NULL,
    text_body    TEXT        NOT NULL,
    updated_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT email_template_versions_unique UNIQUE (template_id, version)
);
CREATE INDEX idx_email_template_versions_template ON email_template_versions(template_id);

-- Seed default templates (mirrors the strings previously hardcoded in internal/mailer).
INSERT INTO email_templates (template_key, locale, subject, html_body, text_body, variables) VALUES
('welcome', 'en',
 'Welcome to MemPan!',
 E'<!DOCTYPE html><html><body>\n<h2>Welcome to MemPan, {{.Username}}!</h2>\n<p>Your account has been created. Start building your flashcard decks today.</p>\n</body></html>',
 E'Welcome to MemPan, {{.Username}}!\n\nYour account has been created. Start building your flashcard decks today.\n',
 '["Username"]'::jsonb),
('email_verification', 'en',
 'Verify your MemPan email',
 E'<!DOCTYPE html><html><body>\n<h2>Verify your email, {{.Username}}</h2>\n<p>Click the link below to verify your email address. This link expires in 24 hours.</p>\n<p><a href="{{.URL}}">Verify Email</a></p>\n<p>Or copy this link: {{.URL}}</p>\n</body></html>',
 E'Verify your email, {{.Username}}\n\nOpen this link to verify your email address (expires in 24 hours):\n{{.URL}}\n',
 '["Username","URL"]'::jsonb),
('password_reset', 'en',
 'Reset your MemPan password',
 E'<!DOCTYPE html><html><body>\n<h2>Reset your password, {{.Username}}</h2>\n<p>We received a request to reset your MemPan password. Click the link below (valid for 1 hour).</p>\n<p><a href="{{.URL}}">Reset Password</a></p>\n<p>If you did not request this, you can ignore this email.</p>\n</body></html>',
 E'Reset your password, {{.Username}}\n\nWe received a request to reset your MemPan password. Open this link (valid for 1 hour):\n{{.URL}}\n\nIf you did not request this, you can ignore this email.\n',
 '["Username","URL"]'::jsonb),
('study_reminder', 'en',
 'Your MemPan study session is waiting',
 E'<!DOCTYPE html><html><body>\n<h2>Time to study, {{.Username}}!</h2>\n<p>You have {{.DueCount}} card(s) due for review today. Keep your streak alive.</p>\n<p><a href="{{.URL}}">Open MemPan</a></p>\n</body></html>',
 E'Time to study, {{.Username}}!\n\nYou have {{.DueCount}} card(s) due for review today. Keep your streak alive.\n{{.URL}}\n',
 '["Username","DueCount","URL"]'::jsonb);
