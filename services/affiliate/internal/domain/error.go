package domain

import "errors"

var (
	ErrReferralAlreadyExists = errors.New("referral already exists")
	ErrMetricsNotFound       = errors.New("agent metrics not found")
	ErrMoneyBackfillRequired = errors.New("legacy affiliate volume requires an explicit currency backfill")
)
