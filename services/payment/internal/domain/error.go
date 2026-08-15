package domain

import "errors"

var (
	ErrTransactionNotFound           = errors.New("transaction not found")
	ErrTransactionAlreadyDone        = errors.New("transaction already processed")
	ErrPaymentFailed                 = errors.New("payment failed")
	ErrPaymentInProgress             = errors.New("payment is already being processed")
	ErrIdempotencyConflict           = errors.New("idempotency key conflicts with the existing payment attempt")
	ErrInitiationIdempotencyConflict = errors.New("initiation idempotency key was already used with a different request")
	ErrInitiationIdempotencyRequired = errors.New("initiation idempotency key is required")
)
