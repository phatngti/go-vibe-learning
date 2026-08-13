package saga

import (
	"context"
	"errors"
	"time"

	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/outbox"
	processcore "github/master_go_cqrs/services/orchestrator/internal/core/process"
	"github/master_go_cqrs/services/orchestrator/internal/payment"
	"github/master_go_cqrs/services/orchestrator/internal/payment/codec"
	paymentsaga "github/master_go_cqrs/services/orchestrator/internal/payment/saga/model"
	paymentpostgres "github/master_go_cqrs/services/orchestrator/internal/payment/saga/repository/postgres"

	"gorm.io/gorm"
)

type Process struct {
	repo       *paymentpostgres.Repository
	outbox     outbox.Repository
	definition payment.Definition
	codec      *codec.Kafka
}

func NewProcess(repo *paymentpostgres.Repository, outboxRepo outbox.Repository, kafkaCodec *codec.Kafka) *Process {
	return &Process{repo: repo, outbox: outboxRepo, definition: payment.NewDefinition(), codec: kafkaCodec}
}

func (p *Process) Name() string { return "payment" }

func (p *Process) TopicFor(eventType string) string { return p.codec.TopicFor(eventType) }

func (p *Process) ProcessEvent(ctx context.Context, tx *gorm.DB, message *kafka.Message) error {
	event, err := p.codec.Decode(message)
	if err != nil {
		return processError(err)
	}
	model, err := p.repo.GetForUpdate(ctx, tx, event.TransactionID)
	if err != nil {
		return err
	}
	var current *payment.Snapshot
	if model != nil {
		snapshot := model.Snapshot()
		current = &snapshot
	}
	transition, err := p.definition.Apply(current, event)
	if err != nil {
		return processError(err)
	}
	if !transition.Changed && transition.Command == nil {
		return nil
	}
	if model == nil {
		model = &paymentsaga.Model{}
	}
	paymentsaga.ApplySnapshot(model, transition.Snapshot)
	if transition.Command != nil {
		model.RetryCount = 1
		next := time.Now().Add(p.definition.Policy().StepTimeout)
		model.NextRetryAt = &next
	} else {
		model.RetryCount = 0
		model.NextRetryAt = nil
	}
	if err := p.repo.Save(ctx, tx, model); err != nil {
		return err
	}
	return p.enqueue(ctx, tx, transition.Command)
}

func (p *Process) Due(ctx context.Context, limit int) ([]string, error) {
	return p.repo.DueIDs(ctx, limit)
}

func (p *Process) Retry(ctx context.Context, transactionID string, now time.Time) error {
	return p.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model, err := p.repo.GetForUpdate(ctx, tx, transactionID)
		if err != nil || model == nil || model.NextRetryAt == nil || model.NextRetryAt.After(now) {
			return err
		}
		transition, err := p.definition.OnTimeout(model.Snapshot(), model.RetryCount)
		if err != nil {
			return err
		}
		paymentsaga.ApplySnapshot(model, transition.Snapshot)
		if transition.Command == nil {
			model.RetryCount = 0
			model.NextRetryAt = nil
		} else {
			model.RetryCount++
			next := now.Add(p.definition.Policy().StepTimeout)
			model.NextRetryAt = &next
		}
		if err := p.repo.Save(ctx, tx, model); err != nil {
			return err
		}
		return p.enqueue(ctx, tx, transition.Command)
	})
}

func (p *Process) enqueue(ctx context.Context, tx *gorm.DB, command *payment.Command) error {
	if command == nil {
		return nil
	}
	encoded, err := p.codec.Encode(*command)
	if err != nil {
		return err
	}
	return p.outbox.Enqueue(ctx, tx, outbox.EnqueueParams{
		MessageID: encoded.Message.ID, Topic: encoded.Topic, MessageKey: encoded.Key,
		MessageType: encoded.Message.EventType, AggregateType: encoded.Message.AggregateType,
		AggregateID: encoded.Message.AggregateID, Payload: encoded.Message.Payload,
	})
}

func processError(err error) error {
	if errors.Is(err, processcore.ErrInvalidEvent) {
		return kafka.Permanent(err)
	}
	return err
}
