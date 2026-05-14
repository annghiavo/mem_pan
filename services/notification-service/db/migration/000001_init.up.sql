CREATE TABLE fcm_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL,
    token       TEXT        NOT NULL,
    device_name TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fcm_tokens_token_key UNIQUE (token)
);
CREATE INDEX idx_fcm_tokens_user_id ON fcm_tokens(user_id);

CREATE TABLE notification_log (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID,
    notification_type TEXT        NOT NULL,
    channel           TEXT        NOT NULL,
    recipient         TEXT        NOT NULL,
    status            TEXT        NOT NULL DEFAULT 'sent',
    error_message     TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notification_log_user_id    ON notification_log(user_id);
CREATE INDEX idx_notification_log_created_at ON notification_log(created_at);
