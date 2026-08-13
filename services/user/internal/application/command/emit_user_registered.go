package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/services/user/internal/domain"
)

// EmitUserRegisteredHandler emits a UserRegistered domain event after a
// successful registration. This logic was previously embedded in the HTTP
// handler, violating the delivery layer's responsibility boundary.
type EmitUserRegisteredHandler struct {
	eventStore eventstore.Repository
}

func NewEmitUserRegisteredHandler(es eventstore.Repository) *EmitUserRegisteredHandler {
	return &EmitUserRegisteredHandler{eventStore: es}
}

func (h *EmitUserRegisteredHandler) Handle(ctx context.Context, providerUserID, email string) error {
	evt := domain.UserRegistered{
		IdentityProviderUserID: providerUserID,
		Email:                  email,
		OccurredAt:             time.Now(),
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal UserRegistered event: %w", err)
	}

	version, err := h.eventStore.NextVersion(ctx, nil, aggregateType, providerUserID)
	if err != nil {
		return fmt.Errorf("next version: %w", err)
	}

	return h.eventStore.AppendEvent(ctx, nil, &eventstore.DomainEvent{
		AggregateType: aggregateType,
		AggregateID:   providerUserID,
		EventType:     evt.EventType(),
		Payload:       string(payload),
		Version:       version,
	})
}
