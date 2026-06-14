CREATE TABLE creator_payout_accounts (
    creator_id UUID PRIMARY KEY,
    bank_bin TEXT NOT NULL,
    bank_code TEXT NOT NULL,
    bank_short_name TEXT NOT NULL,
    bank_name TEXT NOT NULL,
    bank_logo TEXT,
    account_number TEXT NOT NULL,
    account_name TEXT NOT NULL,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(trim(bank_bin)) > 0),
    CHECK (length(trim(bank_code)) > 0),
    CHECK (length(trim(bank_short_name)) > 0),
    CHECK (length(trim(bank_name)) > 0),
    CHECK (length(trim(account_number)) > 0),
    CHECK (length(trim(account_name)) > 0)
);

CREATE INDEX idx_creator_payout_accounts_updated
    ON creator_payout_accounts(updated_at DESC);
