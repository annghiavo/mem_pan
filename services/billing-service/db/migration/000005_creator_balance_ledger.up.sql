CREATE TABLE creator_withdrawals (
    withdrawal_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL,
    amount_vnd BIGINT NOT NULL CHECK (amount_vnd > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'processing',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    paid_at TIMESTAMPTZ,
    payout_reference_id TEXT,
    payout_idempotency_key TEXT,
    payout_to_bin TEXT,
    payout_to_account_number TEXT,
    payout_to_account_name TEXT,
    payos_payout_id TEXT,
    payos_payout_transaction_id TEXT,
    payos_payout_state TEXT,
    payout_raw_payload JSONB,
    payout_failed_reason TEXT
);

CREATE TABLE creator_balance_transactions (
    transaction_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL,
    source_type VARCHAR(30) NOT NULL,
    source_id TEXT NOT NULL,
    amount_vnd BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'posted',
    pool_month DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (source_type, source_id)
);

CREATE INDEX idx_creator_balance_transactions_creator_status
    ON creator_balance_transactions(creator_id, status, created_at DESC);

CREATE INDEX idx_creator_withdrawals_creator_requested
    ON creator_withdrawals(creator_id, requested_at DESC);
