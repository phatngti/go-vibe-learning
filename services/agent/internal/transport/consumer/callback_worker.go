package consumer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/config"
	"github/master_go_cqrs/services/agent/internal/domain"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var callbackRetrySchedule = [...]time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	time.Hour,
	6 * time.Hour,
}

type CallbackDeliveryRunner struct {
	deliveries port.CallbackDeliveryRepository
	validator  port.CallbackEndpointValidator
	signer     port.CallbackSigner
	dispatcher port.CallbackDispatcher
	eventTopic string
	lease      time.Duration
	now        func() time.Time
}

func NewCallbackDeliveryRunner(
	deliveries port.CallbackDeliveryRepository,
	validator port.CallbackEndpointValidator,
	signer port.CallbackSigner,
	dispatcher port.CallbackDispatcher,
	cfg *config.Config,
) *CallbackDeliveryRunner {
	return &CallbackDeliveryRunner{
		deliveries: deliveries, validator: validator, signer: signer, dispatcher: dispatcher,
		eventTopic: cfg.Kafka.Topics.AgentEvents, lease: cfg.Callback.WorkerLeaseDuration,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (r *CallbackDeliveryRunner) RunNext(ctx context.Context, workerID string) (bool, error) {
	claimedAt := r.now()
	delivery, err := r.deliveries.ClaimNext(ctx, workerID, claimedAt, r.lease)
	if err != nil || delivery == nil {
		return false, err
	}
	endpoint, err := r.validator.Validate(ctx, delivery.CallbackURL)
	if err != nil {
		return true, r.markTerminal(ctx, delivery, err.Error(), r.now())
	}
	signature, err := r.signer.Sign(domain.CallbackSigningInput{
		EventID: delivery.EventID, TenantID: delivery.TenantID,
		TransactionID: delivery.TransactionID, Body: delivery.Payload,
	})
	if err != nil {
		return true, r.markTerminal(ctx, delivery, err.Error(), r.now())
	}
	delivery.KeyVersion = signature.KeyVersion
	status, dispatchErr := r.dispatcher.Dispatch(ctx, port.CallbackDispatchRequest{
		URL: endpoint.URL,
		Headers: map[string]string{
			"Content-Type": "application/json", "Idempotency-Key": delivery.EventID,
			port.CallbackHeaderEventID: delivery.EventID, port.CallbackHeaderTimestamp: signature.Timestamp,
			port.CallbackHeaderKeyVersion:    signature.KeyVersion,
			port.CallbackHeaderContentSHA256: signature.BodySHA256,
			port.CallbackHeaderSignature:     signature.Signature,
		},
		Body: delivery.Payload,
	})
	completedAt := r.now()
	if dispatchErr == nil && status >= http.StatusOK && status < http.StatusMultipleChoices {
		result, resultErr := callbackResultIntent(delivery, r.eventTopic, messagebus.CallbackDispatchedType,
			domain.CallbackDeliveryDelivered, "")
		if resultErr != nil {
			return true, resultErr
		}
		return true, r.deliveries.MarkDelivered(ctx, delivery, result, completedAt)
	}

	reason := fmt.Sprintf("callback returned HTTP %d", status)
	retryable := retryableCallbackStatus(status)
	if dispatchErr != nil {
		reason = dispatchErr.Error()
		retryable = !kafka.IsPermanent(dispatchErr)
	}
	if retryable && delivery.GenerationAttempts < len(callbackRetrySchedule) {
		next := completedAt.Add(callbackRetrySchedule[delivery.GenerationAttempts])
		return true, r.deliveries.MarkRetry(ctx, delivery, reason, next, completedAt)
	}
	return true, r.markTerminal(ctx, delivery, reason, completedAt)
}

func (r *CallbackDeliveryRunner) markTerminal(
	ctx context.Context,
	delivery *domain.CallbackDelivery,
	reason string,
	now time.Time,
) error {
	result, err := callbackResultIntent(delivery, r.eventTopic, messagebus.CallbackDispatchFailedType,
		domain.CallbackDeliveryTerminalFailed, reason)
	if err != nil {
		return err
	}
	return r.deliveries.MarkTerminal(ctx, delivery, reason, result, now)
}

func retryableCallbackStatus(status int) bool {
	return status == 0 || status == http.StatusRequestTimeout || status == http.StatusTooEarly ||
		status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

type CallbackDeliveryWorker struct {
	runner      *CallbackDeliveryRunner
	logger      logger.Factory
	enabled     bool
	parallelism int
	poll        time.Duration
	workerBase  string
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewCallbackDeliveryWorker(runner *CallbackDeliveryRunner, cfg *config.Config, log logger.Factory) *CallbackDeliveryWorker {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "agent-service"
	}
	return &CallbackDeliveryWorker{
		runner: runner, logger: log, enabled: cfg.Callback.WorkerEnabled,
		parallelism: cfg.Callback.WorkerParallelism, poll: cfg.Callback.WorkerPollInterval,
		workerBase: hostname + ":" + uuid.NewString(),
	}
}

func (w *CallbackDeliveryWorker) Start(lifecycle fx.Lifecycle) {
	if !w.enabled {
		w.logger.Bg().Info("Durable callback delivery worker disabled")
		return
	}
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			w.cancel = cancel
			for lane := 0; lane < w.parallelism; lane++ {
				w.wg.Add(1)
				go w.run(ctx, fmt.Sprintf("%s:%d", w.workerBase, lane))
			}
			return nil
		},
		OnStop: func(context.Context) error {
			if w.cancel != nil {
				w.cancel()
			}
			w.wg.Wait()
			return nil
		},
	})
}

func (w *CallbackDeliveryWorker) run(ctx context.Context, workerID string) {
	defer w.wg.Done()
	for {
		claimed, err := w.runner.RunNext(ctx, workerID)
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return
		}
		if err != nil {
			w.logger.For(ctx).Error("Durable callback delivery failed").WithFields(
				zap.String("worker_id", workerID), zap.Error(err),
			)
		}
		if claimed && err == nil {
			continue
		}
		if !waitForCallbackPoll(ctx, w.poll) {
			return
		}
	}
}

func waitForCallbackPoll(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
