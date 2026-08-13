package domain

import "errors"

var (
	ErrTransactionNotFound    = errors.New("transaction not found")
	ErrTransactionAlreadyDone = errors.New("transaction already processed")
	ErrPaymentFailed          = errors.New("payment failed")
)
