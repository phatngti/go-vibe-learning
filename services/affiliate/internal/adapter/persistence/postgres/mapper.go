package postgres

import "github/master_go_cqrs/services/affiliate/internal/domain"

func referralToRow(e *domain.Referral) *ReferralRow {
	return &ReferralRow{ReferrerID: e.ReferrerID, RefereeID: e.RefereeID, CreatedAt: e.CreatedAt}
}
func metricToDomain(r *AgentMetricRow) *domain.AgentMetric {
	return &domain.AgentMetric{AgentID: r.AgentID, AccumulatedVolume: r.AccumulatedVolume, ReferralCount: r.ReferralCount, UpdatedAt: r.UpdatedAt}
}
func metricToRow(e *domain.AgentMetric) *AgentMetricRow {
	return &AgentMetricRow{AgentID: e.AgentID, AccumulatedVolume: e.AccumulatedVolume, ReferralCount: e.ReferralCount, UpdatedAt: e.UpdatedAt}
}
func levelToDomain(r *AffiliateLevelRow) *domain.AffiliateLevel {
	return &domain.AffiliateLevel{AgentID: r.AgentID, CurrentRank: domain.Rank(r.CurrentRank), PromotedAt: r.PromotedAt}
}
func levelToRow(e *domain.AffiliateLevel) *AffiliateLevelRow {
	return &AffiliateLevelRow{AgentID: e.AgentID, CurrentRank: string(e.CurrentRank), PromotedAt: e.PromotedAt}
}
