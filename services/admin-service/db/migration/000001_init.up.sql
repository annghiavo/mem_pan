-- ============================================
-- admin_db — admin-service
-- ============================================

CREATE TYPE report_target_type AS ENUM ('deck', 'user', 'note');
CREATE TYPE report_status AS ENUM ('pending', 'reviewing', 'resolved', 'dismissed');
CREATE TYPE report_category AS ENUM (
    'inappropriate_content',
    'copyright_violation',
    'spam',
    'harassment',
    'misinformation',
    'other'
);

-- Reports
CREATE TABLE reports (
    report_id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id         UUID NOT NULL,              -- reference auth_db.users
    target_type         report_target_type NOT NULL,
    target_id           UUID NOT NULL,              -- deck_id / user_id / note_id
    
    reason_category     report_category NOT NULL,
    description         TEXT,
    status              report_status NOT NULL DEFAULT 'pending',
    
    -- Processing
    assigned_to         UUID,                       -- admin user_id
    admin_note          TEXT,
    resolution          VARCHAR(50),                -- 'banned', 'deleted', 'warned', 'no_action'
    resolved_by         UUID,                       -- admin user_id
    resolved_at         TIMESTAMPTZ,
    
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_reports_status ON reports(status, created_at DESC);
CREATE INDEX idx_reports_target ON reports(target_type, target_id);
CREATE INDEX idx_reports_reporter ON reports(reporter_id);
CREATE INDEX idx_reports_assigned ON reports(assigned_to) WHERE status IN ('pending', 'reviewing');

-- Audit log mọi hành động admin
CREATE TABLE moderation_logs (
    log_id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id            UUID NOT NULL,              -- admin user_id
    action              VARCHAR(50) NOT NULL,       -- 'ban_user', 'unban_user', 'hide_deck', 'resolve_report'
    target_type         VARCHAR(20) NOT NULL,       -- 'user', 'deck', 'report'
    target_id           UUID NOT NULL,
    reason              TEXT,
    metadata            JSONB,                      -- thông tin bổ sung
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_moderation_logs_admin ON moderation_logs(admin_id, created_at DESC);
CREATE INDEX idx_moderation_logs_target ON moderation_logs(target_type, target_id);
CREATE INDEX idx_moderation_logs_created ON moderation_logs(created_at DESC);
