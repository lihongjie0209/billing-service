package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lihongjie0209/billing-service/internal/config"
	platformeventbus "github.com/lihongjie0209/microservice-platform-go/eventbus"
	"github.com/lihongjie0209/microservice-platform-go/operationlog"
	platformoutbox "github.com/lihongjie0209/microservice-platform-go/outbox"
	"github.com/lihongjie0209/microservice-platform-go/securitylog"
	commonv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/common/v1"
	"go.uber.org/fx"
)

type billingEventRuntime struct {
	config config.Config
	store  *platformoutbox.SQLStore
	logger *slog.Logger
	cancel context.CancelFunc
	wg     sync.WaitGroup
	bus    *platformeventbus.Bus
}

func newBillingOutboxStore(db *sqlx.DB) (*platformoutbox.SQLStore, error) {
	if db == nil {
		return nil, nil
	}
	return platformoutbox.NewSQLStore(db, "billing_outbox_events", platformoutbox.WithWorkerAuditActor("billing-service:outbox"))
}
func newBillingEventRuntime(lc fx.Lifecycle, cfg config.Config, store *platformoutbox.SQLStore, logger *slog.Logger) *billingEventRuntime {
	r := &billingEventRuntime{config: cfg, store: store, logger: logger}
	lc.Append(fx.Hook{OnStart: r.start, OnStop: r.stop})
	return r
}
func (r *billingEventRuntime) start(ctx context.Context) error {
	if !r.config.EventBus.Enabled {
		return nil
	}
	if r.store == nil {
		return errors.New("enabled event bus requires database outbox")
	}
	bus, err := platformeventbus.New(ctx, platformeventbus.Config{URLs: r.config.EventBus.URLs, ClientName: r.config.App.Name, StreamName: "PLATFORM_EVENTS", Subjects: []string{"platform.>"}, Storage: r.config.EventBus.Storage, MaxAge: r.config.EventBus.MaxAge, DuplicateWindow: r.config.EventBus.DuplicateWindow, ConnectTimeout: r.config.EventBus.ConnectTimeout, PublishTimeout: r.config.EventBus.PublishTimeout})
	if err != nil {
		return err
	}
	dispatcher, err := platformoutbox.New(r.store, bus, platformoutbox.Config{BatchSize: r.config.EventBus.DispatchBatchSize, Lease: r.config.EventBus.DispatchLease, RetryDelay: r.config.EventBus.DispatchRetryDelay})
	if err != nil {
		_ = bus.Close()
		return err
	}
	r.bus = bus
	runCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	cleaner, err := platformoutbox.NewRetentionCleaner(r.store, platformoutbox.RetentionConfig{Retention: r.config.EventBus.PublishedRetention, BatchSize: r.config.EventBus.CleanupBatchSize})
	if err != nil {
		cancel()
		_ = bus.Close()
		return err
	}
	r.wg.Go(func() {
		ticker := time.NewTicker(r.config.EventBus.DispatchInterval)
		defer ticker.Stop()
		for {
			if _, runErr := dispatcher.RunOnce(runCtx); runErr != nil && !errors.Is(runErr, context.Canceled) {
				r.logger.ErrorContext(runCtx, "dispatch billing outbox failed", "error", runErr)
			}
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
			}
		}
	})
	r.wg.Go(func() {
		ticker := time.NewTicker(r.config.EventBus.CleanupInterval)
		defer ticker.Stop()
		for {
			if deleted, runErr := cleaner.RunOnce(runCtx); runErr != nil && !errors.Is(runErr, context.Canceled) {
				r.logger.ErrorContext(runCtx, "clean published billing outbox events", "error", runErr)
			} else if deleted > 0 {
				r.logger.InfoContext(runCtx, "published billing outbox events cleaned", "deleted", deleted)
			}
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
			}
		}
	})
	return nil
}
func (r *billingEventRuntime) stop(context.Context) error {
	if r.cancel != nil {
		r.cancel()
		r.wg.Wait()
	}
	if r.bus != nil {
		return r.bus.Close()
	}
	return nil
}
func (r *billingEventRuntime) Publish(ctx context.Context, subject string, envelope *commonv1.EventEnvelope) error {
	if r == nil || r.bus == nil {
		return errors.New("billing event bus is unavailable")
	}
	return r.bus.Publish(ctx, subject, envelope)
}
func newOperationLogRecorder(cfg config.Config, publisher *billingEventRuntime) (operationlog.Recorder, error) {
	return operationlog.New(operationlog.Config{Enabled: cfg.OperationLog.Enabled, Subject: cfg.OperationLog.Subject, MaxPayloadBytes: cfg.OperationLog.MaxPayloadBytes}, publisher)
}
func newSecurityLogRecorder(cfg config.Config, publisher *billingEventRuntime) (securitylog.Recorder, error) {
	return securitylog.New(securitylog.Config{Enabled: cfg.SecurityLog.Enabled, Subject: cfg.SecurityLog.Subject, MaxPayloadBytes: cfg.SecurityLog.MaxPayloadBytes, HashKey: cfg.SecurityLog.HashKey, FailClosed: cfg.SecurityLog.FailClosed}, publisher)
}

var EventBusModule = fx.Module("billing-event-bus", fx.Provide(newBillingOutboxStore, newBillingEventRuntime, newOperationLogRecorder, newSecurityLogRecorder), fx.Invoke(func(*billingEventRuntime) {}))
