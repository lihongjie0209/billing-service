package scheduler

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

type transitionStub struct {
	calls int
	limit int
}

func (s *transitionStub) ApplyDueSubscriptionTransitions(_ context.Context, limit int) (int, error) {
	s.calls++
	s.limit = limit
	return 2, nil
}

func TestRunSubscriptionTransitionsWithoutRedis(t *testing.T) {
	t.Parallel()
	service := &transitionStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := runSubscriptionTransitions(nil, service, logger); err != nil {
		t.Fatal(err)
	}
	if service.calls != 1 || service.limit != 100 {
		t.Fatalf("calls=%d limit=%d", service.calls, service.limit)
	}
}
