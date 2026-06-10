CREATE TABLE study_session_metrics (
    session_id UUID PRIMARY KEY REFERENCES study_sessions(session_id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    deck_id UUID NOT NULL,
    creator_id UUID,
    card_views INTEGER NOT NULL DEFAULT 0,
    reviewed_cards INTEGER NOT NULL DEFAULT 0,
    total_active_ms BIGINT NOT NULL DEFAULT 0,
    max_gap_ms INTEGER NOT NULL DEFAULT 0,
    weighted_score NUMERIC(12,4) NOT NULL DEFAULT 0,
    is_revshare_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    invalid_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_study_session_metrics_month
    ON study_session_metrics(created_at, creator_id)
    WHERE is_revshare_eligible = TRUE;

CREATE TABLE monthly_revenue_pools (
    pool_month DATE PRIMARY KEY,
    gross_amount_vnd BIGINT NOT NULL,
    creator_pool_amount_vnd BIGINT NOT NULL,
    platform_amount_vnd BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    finalized_at TIMESTAMPTZ
);

CREATE TABLE creator_earnings (
    earning_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_month DATE NOT NULL REFERENCES monthly_revenue_pools(pool_month) ON DELETE CASCADE,
    creator_id UUID NOT NULL,
    eligible_learners INTEGER NOT NULL,
    weighted_score NUMERIC(14,4) NOT NULL,
    amount_vnd BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(pool_month, creator_id)
);
