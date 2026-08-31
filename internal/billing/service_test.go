package billing

import (
	"context"
	"database/sql"
	"math"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
)

type previewRepository struct {
	Repository
	subscription Subscription
	plan         Plan
	prices       []UsagePrice
}

type transactionStub struct{}

func (transactionStub) Within(_ context.Context, _ *sql.TxOptions, fn func(*sqlx.Tx) error) error {
	return fn(nil)
}

type paymentRepository struct {
	Repository
	payment        PaymentAttempt
	invoice        Invoice
	claim          bool
	updatedPayment PaymentAttempt
	updatedInvoice Invoice
}

func (r *paymentRepository) GetPayment(context.Context, string, string) (PaymentAttempt, error) {
	return r.payment, nil
}
func (r *paymentRepository) GetPaymentByKey(_ context.Context, key string) (PaymentAttempt, error) {
	if r.payment.IdempotencyKey == key {
		return r.payment, nil
	}
	return PaymentAttempt{}, ErrNotFound
}
func (r *paymentRepository) ClaimPayment(_ context.Context, _ sqlx.ExtContext, v PaymentAttempt) (string, bool, error) {
	r.payment = v
	return v.ID, true, nil
}
func (r *paymentRepository) GetInvoice(context.Context, string, string) (Invoice, []InvoiceLine, error) {
	return r.invoice, nil, nil
}
func (r *paymentRepository) ClaimProviderEvent(context.Context, sqlx.ExtContext, string, string, string, Audit) (bool, error) {
	return r.claim, nil
}
func (r *paymentRepository) UpdatePayment(_ context.Context, _ sqlx.ExtContext, v PaymentAttempt, _ int64) error {
	r.updatedPayment = v
	return nil
}
func (r *paymentRepository) UpdateInvoice(_ context.Context, _ sqlx.ExtContext, v Invoice, _ int64) error {
	r.updatedInvoice = v
	return nil
}
func (*paymentRepository) AddOutbox(context.Context, sqlx.ExtContext, OutboxEvent) error { return nil }

type paymentGatewayStub struct {
	command PaymentCommand
	result  PaymentGatewayResult
}

func (g *paymentGatewayStub) Create(_ context.Context, command PaymentCommand) (PaymentGatewayResult, error) {
	g.command = command
	return g.result, nil
}
func (*paymentGatewayStub) Reconcile(context.Context, string, time.Time, time.Time, string, uint32) ([]ReconciliationMismatch, string, error) {
	return nil, "", nil
}

func TestCreatePaymentAttemptCallsGatewayAfterClaim(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	repository := &paymentRepository{claim: true, invoice: Invoice{ID: "invoice-1", TenantID: "tenant-1", Currency: "CNY", TotalMinor: 900, Status: "open", Audit: Audit{Version: 1}}}
	gateway := &paymentGatewayStub{result: PaymentGatewayResult{ProviderPaymentID: "provider-payment-1", ProviderEventID: "provider-event-1", Status: "succeeded", ProcessedAt: now}}
	service := NewService(repository, nil, nil)
	service.transactor, service.gateway, service.now = transactionStub{}, gateway, func() time.Time { return now }
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	payment, duplicate, err := service.CreatePaymentAttempt(ctx, "tenant-1", "invoice-1", "demo", "payment-method-secret", "payment-key")
	if err != nil {
		t.Fatal(err)
	}
	if duplicate || payment.Status != "succeeded" || gateway.command.PaymentMethodReference != "payment-method-secret" || gateway.command.AmountMinor != 900 {
		t.Fatalf("payment=%+v duplicate=%v command=%+v", payment, duplicate, gateway.command)
	}
}

func TestCreatePaymentAttemptReplaysCompletedResultAfterInvoicePaid(t *testing.T) {
	t.Parallel()
	repository := &paymentRepository{payment: PaymentAttempt{ID: "payment-1", InvoiceID: "invoice-1", TenantID: "tenant-1", Provider: "demo", IdempotencyKey: "payment-key", RequestHash: hashParts("tenant-1", "invoice-1", "demo", "payment-method-secret"), Status: "succeeded"}, invoice: Invoice{ID: "invoice-1", TenantID: "tenant-1", TotalMinor: 900, PaidMinor: 900, Status: "paid"}}
	service := NewService(repository, nil, nil)
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	payment, duplicate, err := service.CreatePaymentAttempt(ctx, "tenant-1", "invoice-1", "demo", "payment-method-secret", "payment-key")
	if err != nil || !duplicate || payment.Status != "succeeded" {
		t.Fatalf("payment=%+v duplicate=%v err=%v", payment, duplicate, err)
	}
}

func TestApplyPaymentResultMarksInvoicePaid(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 31, 9, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	repository := &paymentRepository{claim: true, payment: PaymentAttempt{ID: "payment-1", InvoiceID: "invoice-1", TenantID: "tenant-1", Provider: "test", AmountMinor: 1200, Status: "pending", Audit: Audit{Version: 1}}, invoice: Invoice{ID: "invoice-1", TenantID: "tenant-1", TotalMinor: 1200, Status: "open", Audit: Audit{Version: 2}}}
	service := NewService(repository, nil, nil)
	service.transactor = transactionStub{}
	service.now = func() time.Time { return now }
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "provider-worker", Type: platformprincipal.TypeServiceAccount})
	payment, invoice, duplicate, err := service.ApplyPaymentResult(ctx, "payment-1", "provider-1", "event-1", "succeeded", "", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate || payment.Status != "succeeded" || invoice.Status != "paid" || invoice.PaidMinor != 1200 || repository.updatedInvoice.Version != 3 {
		t.Fatalf("payment=%+v invoice=%+v duplicate=%v", payment, invoice, duplicate)
	}
}

func TestApplyPaymentResultTreatsProviderReplayAsDuplicate(t *testing.T) {
	t.Parallel()
	repository := &paymentRepository{claim: false, payment: PaymentAttempt{ID: "payment-1", InvoiceID: "invoice-1", TenantID: "tenant-1", Provider: "test", AmountMinor: 100, Status: "succeeded", Audit: Audit{Version: 2}}, invoice: Invoice{ID: "invoice-1", TenantID: "tenant-1", TotalMinor: 100, PaidMinor: 100, Status: "paid", Audit: Audit{Version: 2}}}
	service := NewService(repository, nil, nil)
	service.transactor = transactionStub{}
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "provider-worker", Type: platformprincipal.TypeServiceAccount})
	_, _, duplicate, err := service.ApplyPaymentResult(ctx, "payment-1", "provider-1", "event-1", "succeeded", "", "", time.Now())
	if err != nil || !duplicate {
		t.Fatalf("duplicate=%v err=%v", duplicate, err)
	}
	if repository.updatedPayment.ID != "" || repository.updatedInvoice.ID != "" {
		t.Fatal("duplicate callback must not mutate state")
	}
}

func TestRecordRefundRejectsAmountAboveRefundableBalance(t *testing.T) {
	t.Parallel()
	repository := &paymentRepository{payment: PaymentAttempt{ID: "payment-1", InvoiceID: "invoice-1", TenantID: "tenant-1", Status: "succeeded"}, invoice: Invoice{ID: "invoice-1", TenantID: "tenant-1", PaidMinor: 1000, RefundedMinor: 800}}
	service := NewService(repository, nil, nil)
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	if _, _, _, err := service.RecordRefund(ctx, "tenant-1", "payment-1", "provider-refund", "refund-key", 201, "duplicate", "succeeded"); err == nil {
		t.Fatal("expected over-refund rejection")
	}
}

func TestValidPaymentTransitionRejectsTerminalRegression(t *testing.T) {
	t.Parallel()
	if !validPaymentTransition("pending", "requires_action") || !validPaymentTransition("requires_action", "succeeded") {
		t.Fatal("expected forward transitions")
	}
	if validPaymentTransition("succeeded", "pending") || validPaymentTransition("failed", "succeeded") {
		t.Fatal("terminal payment status must not regress")
	}
}

func (r *previewRepository) GetSubscription(context.Context, string, string) (Subscription, error) {
	return r.subscription, nil
}
func (r *previewRepository) GetPlan(context.Context, string, string) (Plan, error) {
	return r.plan, nil
}
func (r *previewRepository) ListUsagePrices(context.Context, string) ([]UsagePrice, error) {
	return r.prices, nil
}

type usageStub struct{ quantities map[string]int64 }

func (u usageStub) Total(_ context.Context, _, meter string, _, _ time.Time) (int64, error) {
	return u.quantities[meter], nil
}

func TestPreviewInvoiceCalculatesBaseAndMeteredUsage(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	repository := &previewRepository{
		subscription: Subscription{ID: "subscription-1", TenantID: "tenant-1", PlanID: "plan-1", CurrentPeriodStart: start, CurrentPeriodEnd: start.AddDate(0, 1, 0)},
		plan:         Plan{ID: "plan-1", Name: "Pro", Currency: "CNY", BaseAmountMinor: 1000},
		prices:       []UsagePrice{{MeterCode: "api.calls", IncludedQuantity: 100, UnitQuantity: 10, UnitAmountMinor: 25}},
	}
	service := NewService(repository, nil, usageStub{quantities: map[string]int64{"api.calls": 126}})
	service.now = func() time.Time { return start }
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	preview, err := service.PreviewInvoice(ctx, "tenant-1", "subscription-1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Invoice.TotalMinor != 1075 || len(preview.Lines) != 2 || preview.Lines[1].Quantity != 126 || preview.Lines[1].AmountMinor != 75 {
		t.Fatalf("unexpected preview: %+v lines=%+v", preview.Invoice, preview.Lines)
	}
}
func TestPreviewInvoiceRejectsCrossTenant(t *testing.T) {
	t.Parallel()
	service := NewService(&previewRepository{}, nil, usageStub{})
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	if _, err := service.PreviewInvoice(ctx, "tenant-2", "subscription-1", time.Now(), time.Now().Add(time.Hour)); err == nil {
		t.Fatal("expected tenant access denial")
	}
}
func TestUsageAmountRoundsUpAndChecksOverflow(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ quantity, unit, price, want int64 }{{0, 10, 5, 0}, {1, 10, 5, 5}, {10, 10, 5, 5}, {11, 10, 5, 10}, {math.MaxInt64, math.MaxInt64, 7, 7}} {
		got, err := usageAmount(test.quantity, test.unit, test.price)
		if err != nil || got != test.want {
			t.Fatalf("usageAmount(%d,%d,%d)=%d,%v want %d", test.quantity, test.unit, test.price, got, err, test.want)
		}
	}
	if _, err := usageAmount(math.MaxInt64, 1, 2); err == nil {
		t.Fatal("expected overflow")
	}
}
func TestAddIntervalPreservesCalendarBoundary(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, time.January, 31, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	if got := addInterval(start, "month"); got.Month() != time.February || got.Day() != 28 {
		t.Fatalf("month end must clamp to February 28, got %v", got)
	}
	leapStart := time.Date(2024, time.February, 29, 8, 0, 0, 0, start.Location())
	if got := addInterval(leapStart, "year"); got.Month() != time.February || got.Day() != 28 {
		t.Fatalf("leap year end must clamp to February 28, got %v", got)
	}
}
