-- name: UpsertCreatorEarningCreditTransaction :one
INSERT INTO creator_balance_transactions (
    creator_id,
    source_type,
    source_id,
    amount_vnd,
    status,
    pool_month
)
VALUES (
    sqlc.arg('creator_id'),
    'earning_credit',
    sqlc.arg('source_id'),
    sqlc.arg('amount_vnd'),
    'posted',
    sqlc.arg('pool_month')
)
ON CONFLICT (source_type, source_id) DO UPDATE
SET amount_vnd = EXCLUDED.amount_vnd,
    status = 'posted',
    pool_month = EXCLUDED.pool_month,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: CreateCreatorWithdrawal :one
INSERT INTO creator_withdrawals (
    creator_id,
    amount_vnd,
    status,
    payout_reference_id,
    payout_idempotency_key,
    payout_to_bin,
    payout_to_account_number,
    payout_to_account_name
)
VALUES (
    sqlc.arg('creator_id'),
    sqlc.arg('amount_vnd'),
    sqlc.arg('status'),
    sqlc.arg('payout_reference_id'),
    sqlc.arg('payout_idempotency_key'),
    sqlc.arg('payout_to_bin'),
    sqlc.arg('payout_to_account_number'),
    sqlc.arg('payout_to_account_name')
)
RETURNING *;

-- name: GetCreatorWithdrawalByID :one
SELECT * FROM creator_withdrawals
WHERE withdrawal_id = $1
LIMIT 1;

-- name: UpdateCreatorWithdrawalStatus :one
UPDATE creator_withdrawals
SET status = sqlc.arg('status')::text,
    paid_at = CASE WHEN sqlc.arg('status')::text = 'paid' THEN CURRENT_TIMESTAMP ELSE paid_at END,
    payos_payout_id = COALESCE(sqlc.arg('payos_payout_id'), payos_payout_id),
    payos_payout_transaction_id = COALESCE(sqlc.arg('payos_payout_transaction_id'), payos_payout_transaction_id),
    payos_payout_state = COALESCE(sqlc.arg('payos_payout_state'), payos_payout_state),
    payout_raw_payload = sqlc.arg('payout_raw_payload')::text::jsonb,
    payout_failed_reason = sqlc.arg('payout_failed_reason')
WHERE withdrawal_id = sqlc.arg('withdrawal_id')
RETURNING *;

-- name: UpsertCreatorWithdrawalReservation :one
INSERT INTO creator_balance_transactions (
    creator_id,
    source_type,
    source_id,
    amount_vnd,
    status
)
VALUES (
    sqlc.arg('creator_id'),
    'withdrawal',
    sqlc.arg('source_id'),
    sqlc.arg('amount_vnd'),
    sqlc.arg('status')
)
ON CONFLICT (source_type, source_id) DO UPDATE
SET amount_vnd = EXCLUDED.amount_vnd,
    status = EXCLUDED.status,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpdateCreatorBalanceTransactionStatus :one
UPDATE creator_balance_transactions
SET status = sqlc.arg('status'),
    updated_at = CURRENT_TIMESTAMP
WHERE source_type = sqlc.arg('source_type')
  AND source_id = sqlc.arg('source_id')
RETURNING *;

-- name: GetCreatorBalanceSummary :one
SELECT
    COALESCE(SUM(CASE WHEN cbt.source_type = 'earning_credit' AND cbt.status = 'posted' THEN cbt.amount_vnd ELSE 0 END), 0)::bigint AS total_earned_amount_vnd,
    COALESCE(SUM(CASE WHEN cbt.source_type = 'withdrawal' AND (cbt.status = 'posted' OR w.status = 'paid') THEN -cbt.amount_vnd ELSE 0 END), 0)::bigint AS total_withdrawn_amount_vnd,
    COALESCE(SUM(CASE WHEN cbt.source_type = 'withdrawal' AND cbt.status = 'reserved' AND COALESCE(w.status, '') NOT IN ('paid', 'failed') THEN -cbt.amount_vnd ELSE 0 END), 0)::bigint AS pending_withdrawal_amount_vnd,
    (
        COALESCE(SUM(CASE WHEN cbt.source_type = 'earning_credit' AND cbt.status = 'posted' THEN cbt.amount_vnd ELSE 0 END), 0)
        - COALESCE(SUM(CASE WHEN cbt.source_type = 'withdrawal' AND (cbt.status = 'posted' OR w.status = 'paid') THEN -cbt.amount_vnd ELSE 0 END), 0)
        - COALESCE(SUM(CASE WHEN cbt.source_type = 'withdrawal' AND cbt.status = 'reserved' AND COALESCE(w.status, '') NOT IN ('paid', 'failed') THEN -cbt.amount_vnd ELSE 0 END), 0)
    )::bigint AS available_balance_vnd
FROM creator_balance_transactions cbt
LEFT JOIN creator_withdrawals w
    ON cbt.source_type = 'withdrawal'
   AND cbt.source_id = w.withdrawal_id::text
WHERE cbt.creator_id = $1;

-- name: ListCreatorBalanceHistory :many
SELECT
    cbt.transaction_id,
    cbt.source_type,
    cbt.source_id,
    cbt.amount_vnd,
    cbt.status AS ledger_status,
    cbt.pool_month,
    cbt.created_at,
    cbt.updated_at,
    ce.earning_id,
    ce.eligible_learners,
    ce.weighted_score,
    ce.status AS earning_status,
    w.withdrawal_id,
    w.amount_vnd AS withdrawal_amount_vnd,
    w.status AS withdrawal_status,
    w.requested_at AS withdrawal_requested_at,
    w.paid_at AS withdrawal_paid_at,
    w.payout_to_bin,
    w.payout_to_account_number,
    w.payout_to_account_name,
    w.payos_payout_id,
    w.payos_payout_transaction_id,
    w.payos_payout_state,
    w.payout_failed_reason
FROM creator_balance_transactions cbt
LEFT JOIN creator_earnings ce
    ON cbt.source_type = 'earning_credit'
   AND cbt.source_id = ce.earning_id::text
LEFT JOIN creator_withdrawals w
    ON cbt.source_type = 'withdrawal'
   AND cbt.source_id = w.withdrawal_id::text
WHERE cbt.creator_id = sqlc.arg('creator_id')
ORDER BY COALESCE(w.requested_at, cbt.pool_month, cbt.created_at) DESC,
         cbt.created_at DESC
LIMIT sqlc.arg('limit_rows') OFFSET sqlc.arg('offset_rows');
