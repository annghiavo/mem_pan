ALTER TABLE creator_earnings
    ADD COLUMN payout_reference_id TEXT,
    ADD COLUMN payout_idempotency_key TEXT,
    ADD COLUMN payout_to_bin TEXT,
    ADD COLUMN payout_to_account_number TEXT,
    ADD COLUMN payout_to_account_name TEXT,
    ADD COLUMN payos_payout_id TEXT,
    ADD COLUMN payos_payout_transaction_id TEXT,
    ADD COLUMN payos_payout_state TEXT,
    ADD COLUMN payout_raw_payload JSONB,
    ADD COLUMN payout_requested_at TIMESTAMPTZ,
    ADD COLUMN payout_failed_reason TEXT;

CREATE UNIQUE INDEX idx_creator_earnings_payout_reference_id
    ON creator_earnings(payout_reference_id)
    WHERE payout_reference_id IS NOT NULL;

CREATE UNIQUE INDEX idx_creator_earnings_payout_idempotency_key
    ON creator_earnings(payout_idempotency_key)
    WHERE payout_idempotency_key IS NOT NULL;
