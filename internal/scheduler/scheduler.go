package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/lihongjie0209/billing-service/internal/billing"
	"github.com/lihongjie0209/billing-service/internal/cache"
	"github.com/lihongjie0209/billing-service/internal/config"
	"github.com/lihongjie0209/billing-service/internal/observability"
	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
)

func New(lc fx.Lifecycle, cfg config.Config, locker *cache.Locker, billingService *billing.Service, metrics *observability.Metrics, logger *slog.Logger) (*cron.Cron, error) {
	location, err := time.LoadLocation(cfg.Cron.Timezone)
	if err != nil {
		return nil, err
	}
	runner := cron.New(cron.WithLocation(location), cron.WithSeconds(), cron.WithChain(cron.Recover(cron.PrintfLogger(slogWriter{logger})), cron.SkipIfStillRunning(cron.PrintfLogger(slogWriter{logger}))))
	if cfg.Cron.SampleSpec != "" {
		if _, err := runner.AddFunc(cfg.Cron.SampleSpec, func() {
			started := time.Now()
			status := "success"
			if err := runSubscriptionTransitions(locker, billingService, logger); err != nil {
				status = "error"
			}
			metrics.ObserveCron("subscription_transitions", status, started)
		}); err != nil {
			return nil, err
		}
	}
	lc.Append(fx.Hook{OnStart: func(context.Context) error {
		if cfg.Cron.Enabled {
			runner.Start()
			logger.Info("scheduler started")
		}
		return nil
	}, OnStop: func(ctx context.Context) error {
		stopCtx := runner.Stop()
		select {
		case <-stopCtx.Done():
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}})
	return runner, nil
}

type transitionService interface {
	ApplyDueSubscriptionTransitions(context.Context, int) (int, error)
}

func runSubscriptionTransitions(locker *cache.Locker, service transitionService, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if locker == nil {
		_, err := service.ApplyDueSubscriptionTransitions(ctx, 100)
		return err
	}
	lock, acquired, err := locker.TryLock(ctx, "billing:cron:subscription-transitions", time.Minute)
	if err != nil {
		logger.Error("scheduled job lock failed", "error", err)
		return err
	}
	if !acquired {
		logger.Info("sample scheduled job skipped", "reason", "lock held")
		return nil
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := lock.Unlock(unlockCtx); err != nil {
			logger.Error("scheduled job unlock failed", "error", err)
		}
	}()
	applied, err := service.ApplyDueSubscriptionTransitions(ctx, 100)
	if err == nil {
		logger.Info("subscription transitions applied", "count", applied)
	}
	return err
}

type slogWriter struct{ logger *slog.Logger }

func (w slogWriter) Printf(format string, args ...any) {
	w.logger.Error("scheduler event", "detail", format, "args", args)
}

var Module = fx.Module("scheduler", fx.Provide(New), fx.Invoke(func(*cron.Cron) {}))
