package domain

import "errors"

var (
	ErrReferralAlreadyExists = errors.New("referral already exists")
	ErrMetricsNotFound       = errors.New("agent metrics not found")
)
