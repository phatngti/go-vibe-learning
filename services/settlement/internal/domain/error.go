package domain

import "errors"

var (
	ErrBalanceNotFound       = errors.New("agent balance not found")
	ErrInsufficientBalance   = errors.New("insufficient balance")
	ErrMoneyBackfillRequired = errors.New("legacy settlement balance requires an explicit currency backfill")
)
