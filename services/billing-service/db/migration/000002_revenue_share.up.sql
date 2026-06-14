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
    pool_month DATE NOT NULL REFERENCES monthly_revenue_pools(pool_month),
    creator_id UUID NOT NULL,
    eligible_learners INTEGER NOT NULL,
    weighted_score NUMERIC(14,4) NOT NULL,
    amount_vnd BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(pool_month, creator_id)
);
