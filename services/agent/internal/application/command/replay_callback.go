package command

import (
	"context"
	"time"

	"github/master_go_cqrs/services/agent/internal/application/dto"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type ReplayCallbackHandler struct {
	deliveries port.CallbackDeliveryRepository
	now        func() time.Time
}

func NewReplayCallbackHandler(deliveries port.CallbackDeliveryRepository) *ReplayCallbackHandler {
	return &ReplayCallbackHandler{deliveries: deliveries, now: func() time.Time { return time.Now().UTC() }}
}

func (h *ReplayCallbackHandler) Handle(ctx context.Context, request *dto.ReplayCallback) (*domain.CallbackDelivery, error) {
	if request == nil || !request.PlatformOperator {
		return nil, domain.ErrCallbackReplayForbidden
	}
	return h.deliveries.ReplayTerminal(ctx, port.CallbackReplayRequest{
		EventID: request.EventID, TenantID: request.TenantID, OperatorID: request.OperatorID,
		Reason: request.Reason, AvailableAt: h.now(),
	})
}
