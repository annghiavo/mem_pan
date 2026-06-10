package domain

import "errors"

var (
	ErrInvalidPlan          = errors.New("invalid plan_code")
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrPaymentNotFound      = errors.New("payment transaction not found")
	ErrInvalidWebhook       = errors.New("invalid payos webhook")
	ErrDuplicateWebhook     = errors.New("duplicate webhook")
	ErrAmountMismatch       = errors.New("payment amount mismatch")
)
