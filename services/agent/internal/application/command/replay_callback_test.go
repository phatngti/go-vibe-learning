package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github/master_go_cqrs/services/agent/internal/application/dto"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type callbackReplayRepositoryStub struct {
	request port.CallbackReplayRequest
}

func (*callbackReplayRepositoryStub) Schedule(context.Context, port.ScheduleCallbackParams) (*domain.CallbackDelivery, bool, error) {
	return nil, false, nil
}
func (*callbackReplayRepositoryStub) GetByEventID(context.Context, string) (*domain.CallbackDelivery, error) {
	return nil, nil
}
func (*callbackReplayRepositoryStub) ClaimNext(context.Context, string, time.Time, time.Duration) (*domain.CallbackDelivery, error) {
	return nil, nil
}
func (*callbackReplayRepositoryStub) MarkDelivered(context.Context, *domain.CallbackDelivery, port.CallbackResultIntent, time.Time) error {
	return nil
}
func (*callbackReplayRepositoryStub) MarkRetry(context.Context, *domain.CallbackDelivery, string, time.Time, time.Time) error {
	return nil
}
func (*callbackReplayRepositoryStub) MarkTerminal(context.Context, *domain.CallbackDelivery, string, port.CallbackResultIntent, time.Time) error {
	return nil
}
func (r *callbackReplayRepositoryStub) ReplayTerminal(_ context.Context, request port.CallbackReplayRequest) (*domain.CallbackDelivery, error) {
	r.request = request
	return &domain.CallbackDelivery{EventID: request.EventID, Status: domain.CallbackDeliveryRetryScheduled}, nil
}

func TestReplayCallbackRequiresPlatformOperatorAndUsesVerifiedActor(t *testing.T) {
	repository := &callbackReplayRepositoryStub{}
	handler := NewReplayCallbackHandler(repository)
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	handler.now = func() time.Time { return now }

	request := &dto.ReplayCallback{
		EventID: "event-1", TenantID: "tenant-1", OperatorID: "operator-1", Reason: "endpoint recovered",
	}
	if _, err := handler.Handle(context.Background(), request); !errors.Is(err, domain.ErrCallbackReplayForbidden) {
		t.Fatalf("non-operator error = %v", err)
	}
	request.PlatformOperator = true
	if _, err := handler.Handle(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if repository.request.EventID != "event-1" || repository.request.TenantID != "tenant-1" ||
		repository.request.OperatorID != "operator-1" || !repository.request.AvailableAt.Equal(now) {
		t.Fatalf("replay request = %+v", repository.request)
	}
}

var _ port.CallbackDeliveryRepository = (*callbackReplayRepositoryStub)(nil)
