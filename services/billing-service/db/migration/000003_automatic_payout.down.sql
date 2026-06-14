DROP INDEX IF EXISTS idx_creator_earnings_payout_idempotency_key;
DROP INDEX IF EXISTS idx_creator_earnings_payout_reference_id;

ALTER TABLE creator_earnings
    DROP COLUMN IF EXISTS payout_failed_reason,
    DROP COLUMN IF EXISTS payout_requested_at,
    DROP COLUMN IF EXISTS payout_raw_payload,
    DROP COLUMN IF EXISTS payos_payout_state,
    DROP COLUMN IF EXISTS payos_payout_transaction_id,
    DROP COLUMN IF EXISTS payos_payout_id,
    DROP COLUMN IF EXISTS payout_to_account_name,
    DROP COLUMN IF EXISTS payout_to_account_number,
    DROP COLUMN IF EXISTS payout_to_bin,
    DROP COLUMN IF EXISTS payout_idempotency_key,
    DROP COLUMN IF EXISTS payout_reference_id;
