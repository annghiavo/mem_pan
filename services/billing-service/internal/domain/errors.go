package domain

import "errors"

var (
	ErrInvalidPlan           = errors.New("invalid plan_code")
	ErrSubscriptionNotFound  = errors.New("subscription not found")
	ErrPaymentNotFound       = errors.New("payment transaction not found")
	ErrInvalidWebhook        = errors.New("invalid payos webhook")
	ErrDuplicateWebhook      = errors.New("duplicate webhook")
	ErrAmountMismatch        = errors.New("payment amount mismatch")
	ErrEarningNotFound       = errors.New("creator earning not found")
	ErrInvalidPayout         = errors.New("invalid payout request")
	ErrPayoutAmountTooSmall  = errors.New("payout amount must be over 100000 VND")
	ErrPayoutNotAllowed      = errors.New("creator earning is not eligible for payout")
	ErrPayoutForbidden       = errors.New("creator earning does not belong to user")
	ErrPayoutAccountNotFound = errors.New("creator payout account not found")
	ErrInsufficientBalance   = errors.New("insufficient creator balance")
	ErrWithdrawalNotFound    = errors.New("creator withdrawal not found")
)
