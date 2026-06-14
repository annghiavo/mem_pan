-- name: UpsertCreatorPayoutAccount :one
INSERT INTO creator_payout_accounts (
    creator_id,
    bank_bin,
    bank_code,
    bank_short_name,
    bank_name,
    bank_logo,
    account_number,
    account_name,
    verified_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, NULL
)
ON CONFLICT (creator_id) DO UPDATE
SET bank_bin = EXCLUDED.bank_bin,
    bank_code = EXCLUDED.bank_code,
    bank_short_name = EXCLUDED.bank_short_name,
    bank_name = EXCLUDED.bank_name,
    bank_logo = EXCLUDED.bank_logo,
    account_number = EXCLUDED.account_number,
    account_name = EXCLUDED.account_name,
    verified_at = NULL,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetCreatorPayoutAccount :one
SELECT * FROM creator_payout_accounts
WHERE creator_id = $1
LIMIT 1;
