package billing

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lihongjie0209/billing-service/internal/apperror"
	"github.com/lihongjie0209/microservice-platform-go/appaccess"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	commonv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/common/v1"
	"google.golang.org/protobuf/proto"
)

type allowApplicationVerifier struct{}

func (allowApplicationVerifier) Verify(context.Context, string, string) error { return nil }

type rejectingApplicationVerifier struct{ err error }

func (v rejectingApplicationVerifier) Verify(context.Context, string, string) error { return v.err }

func newTestService(t *testing.T, repository Repository, usage UsageReader) *Service {
	t.Helper()
	service, err := NewService(repository, nil, usage, allowApplicationVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

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

type planImportRepository struct {
	Repository
	plan    *Plan
	creates int
}

type subscriptionRepository struct {
	Repository
	outbox OutboxEvent
}

type usagePriceRepository struct {
	Repository
	lockedPlanID      string
	lockedPlanVersion int64
	upserts           int
}

type subscriptionChangeRepository struct {
	Repository
	lockedPlanID      string
	lockedPlanVersion int64
	updated           Subscription
}

func (r *subscriptionChangeRepository) GetSubscription(_ context.Context, tenantID, applicationID, id string) (Subscription, error) {
	return Subscription{
		ID: id, TenantID: tenantID, ApplicationID: applicationID, PlanID: "plan-old", Status: "active",
		CurrentPeriodEnd: time.Now().AddDate(0, 1, 0), Audit: Audit{Version: 4},
	}, nil
}

func (r *subscriptionChangeRepository) LockActivePlan(_ context.Context, _ sqlx.ExtContext, id string, version int64) (Plan, error) {
	r.lockedPlanID, r.lockedPlanVersion = id, version
	return Plan{ID: id, Status: PlanStatusActive, Audit: Audit{Version: version}}, nil
}

func (r *subscriptionChangeRepository) UpdateSubscription(_ context.Context, _ sqlx.ExtContext, value Subscription, _ int64) error {
	r.updated = value
	return nil
}

func (*subscriptionChangeRepository) AddOutbox(context.Context, sqlx.ExtContext, OutboxEvent) error {
	return nil
}

func (r *usagePriceRepository) LockActivePlan(_ context.Context, _ sqlx.ExtContext, id string, version int64) (Plan, error) {
	r.lockedPlanID, r.lockedPlanVersion = id, version
	return Plan{ID: id, Status: PlanStatusActive, Audit: Audit{Version: version}}, nil
}

func (r *usagePriceRepository) UpsertUsagePrice(context.Context, sqlx.ExtContext, UsagePrice, int64) error {
	r.upserts++
	return nil
}

type paymentListRepository struct {
	Repository
	paymentTenantID      string
	paymentApplicationID string
	paymentID            string
	refundTenantID       string
	refundApplicationID  string
}

func (r *paymentListRepository) GetPayment(_ context.Context, tenantID, applicationID, id string) (PaymentAttempt, error) {
	r.paymentTenantID, r.paymentApplicationID, r.paymentID = tenantID, applicationID, id
	return PaymentAttempt{ID: id, TenantID: tenantID, ApplicationID: applicationID}, nil
}

func (r *paymentListRepository) ListPayments(_ context.Context, tenantID, applicationID, _ string, _, _ int) ([]PaymentAttempt, int64, error) {
	r.paymentTenantID, r.paymentApplicationID = tenantID, applicationID
	return []PaymentAttempt{{ID: "payment-1", TenantID: tenantID, ApplicationID: applicationID}}, 1, nil
}

func (r *paymentListRepository) ListRefunds(_ context.Context, tenantID, applicationID, _ string, _, _ int) ([]Refund, int64, error) {
	r.refundTenantID, r.refundApplicationID = tenantID, applicationID
	return []Refund{{ID: "refund-1", TenantID: tenantID, ApplicationID: applicationID}}, 1, nil
}

func TestListPaymentsAndRefundsPreserveApplicationScope(t *testing.T) {
	t.Parallel()
	repository := &paymentListRepository{}
	service := newTestService(t, repository, nil)
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{
		ID:       "user-1",
		Type:     platformprincipal.TypeUser,
		TenantID: "tenant-1",
	})

	payments, err := service.ListPayments(ctx, "tenant-1", "app-1", "succeeded", 1, 20)
	if err != nil || payments.Total != 1 || len(payments.Items) != 1 {
		t.Fatalf("ListPayments() = %+v, %v", payments, err)
	}
	payment, err := service.GetPayment(ctx, "tenant-1", "app-1", " payment-1 ")
	if err != nil || payment.ID != "payment-1" {
		t.Fatalf("GetPayment() = %+v, %v", payment, err)
	}
	refunds, err := service.ListRefunds(ctx, "tenant-1", "app-1", "succeeded", 1, 20)
	if err != nil || refunds.Total != 1 || len(refunds.Items) != 1 {
		t.Fatalf("ListRefunds() = %+v, %v", refunds, err)
	}
	if repository.paymentTenantID != "tenant-1" || repository.paymentApplicationID != "app-1" ||
		repository.paymentID != "payment-1" || repository.refundTenantID != "tenant-1" || repository.refundApplicationID != "app-1" {
		t.Fatalf("repository scopes = payment %s/%s refund %s/%s", repository.paymentTenantID, repository.paymentApplicationID, repository.refundTenantID, repository.refundApplicationID)
	}
}

func (*subscriptionRepository) LockActivePlan(context.Context, sqlx.ExtContext, string, int64) (Plan, error) {
	return Plan{ID: "plan-1", Status: "active", BillingInterval: "month", Audit: Audit{Version: 1}}, nil
}
func (*subscriptionRepository) ClaimSubscription(context.Context, sqlx.ExtContext, string, string, string, Audit) error {
	return nil
}
func (*subscriptionRepository) CreateSubscription(context.Context, sqlx.ExtContext, Subscription) error {
	return nil
}
func (r *subscriptionRepository) AddOutbox(_ context.Context, _ sqlx.ExtContext, event OutboxEvent) error {
	r.outbox = event
	return nil
}

func TestCreateSubscriptionPublishesApplicationScopedEvent(t *testing.T) {
	t.Parallel()
	repository := &subscriptionRepository{}
	service := newTestService(t, repository, nil)
	service.transactor = transactionStub{}
	service.now = func() time.Time {
		return time.Date(2026, time.September, 1, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	}
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	subscription, err := service.CreateSubscription(ctx, "tenant-1", "app-1", "plan-1", 1, time.Time{}, "")
	if err != nil {
		t.Fatal(err)
	}
	envelope := &commonv1.EventEnvelope{}
	if err := proto.Unmarshal(repository.outbox.Envelope, envelope); err != nil {
		t.Fatal(err)
	}
	if subscription.ApplicationID != "app-1" || envelope.GetTenantId() != "tenant-1" || envelope.GetApplicationId() != "app-1" {
		t.Fatalf("subscription=%+v envelope scope=%s/%s", subscription, envelope.GetTenantId(), envelope.GetApplicationId())
	}
}

func TestUpsertUsagePriceLocksCurrentActivePlan(t *testing.T) {
	t.Parallel()
	repository := &usagePriceRepository{}
	service := newTestService(t, repository, nil)
	service.transactor = transactionStub{}
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser})

	value, err := service.UpsertUsagePrice(ctx, UsagePrice{
		PlanID: "plan-1", MeterCode: "requests", UnitQuantity: 1, PricingModel: "per_unit", TiersJSON: "[]",
	}, 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if value.ID == "" || repository.lockedPlanID != "plan-1" || repository.lockedPlanVersion != 3 || repository.upserts != 1 {
		t.Fatalf("value=%+v lock=%s/%d upserts=%d", value, repository.lockedPlanID, repository.lockedPlanVersion, repository.upserts)
	}
}

func TestUpsertUsagePriceRequiresPlanVersion(t *testing.T) {
	t.Parallel()
	repository := &usagePriceRepository{}
	service := newTestService(t, repository, nil)
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser})

	_, err := service.UpsertUsagePrice(ctx, UsagePrice{
		PlanID: "plan-1", MeterCode: "requests", UnitQuantity: 1, PricingModel: "per_unit", TiersJSON: "[]",
	}, 0, 0)
	if err == nil || repository.upserts != 0 {
		t.Fatalf("error=%v upserts=%d", err, repository.upserts)
	}
}

func TestChangeSubscriptionLocksTargetPlanVersion(t *testing.T) {
	t.Parallel()
	repository := &subscriptionChangeRepository{}
	service := newTestService(t, repository, nil)
	service.transactor = transactionStub{}
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{
		ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1",
	})

	value, err := service.ChangeSubscription(ctx, "tenant-1", "app-1", "subscription-1", " plan-new ", "immediate", 4, 9)
	if err != nil {
		t.Fatal(err)
	}
	if value.PlanID != "plan-new" || repository.lockedPlanID != "plan-new" || repository.lockedPlanVersion != 9 || repository.updated.Version != 5 {
		t.Fatalf("value=%+v lock=%s/%d updated=%+v", value, repository.lockedPlanID, repository.lockedPlanVersion, repository.updated)
	}
}

func TestChangeSubscriptionRequiresTargetPlanVersion(t *testing.T) {
	t.Parallel()
	repository := &subscriptionChangeRepository{}
	service := newTestService(t, repository, nil)
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{
		ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1",
	})

	_, err := service.ChangeSubscription(ctx, "tenant-1", "app-1", "subscription-1", "plan-new", "immediate", 4, 0)
	if err == nil || repository.lockedPlanID != "" {
		t.Fatalf("error=%v locked plan=%q", err, repository.lockedPlanID)
	}
}

func TestCreateSubscriptionRequiresPlanVersion(t *testing.T) {
	t.Parallel()
	repository := &subscriptionRepository{}
	service := newTestService(t, repository, nil)
	service.transactor = transactionStub{}
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})

	_, err := service.CreateSubscription(ctx, "tenant-1", "app-1", "plan-1", 0, time.Time{}, "")
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeInvalidArgument {
		t.Fatalf("CreateSubscription() error = %v, want invalid argument", err)
	}
}

func (r *planImportRepository) GetPlan(_ context.Context, _, code string) (Plan, error) {
	if r.plan != nil && r.plan.Code == code {
		return *r.plan, nil
	}
	return Plan{}, ErrNotFound
}
func (r *planImportRepository) CreatePlan(_ context.Context, _ sqlx.ExtContext, value Plan) error {
	r.creates++
	r.plan = &value
	return nil
}
func (*planImportRepository) AddOutbox(context.Context, sqlx.ExtContext, OutboxEvent) error {
	return nil
}

func (r *paymentRepository) GetPayment(context.Context, string, string, string) (PaymentAttempt, error) {
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
func (r *paymentRepository) GetInvoice(context.Context, string, string, string) (Invoice, []InvoiceLine, error) {
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
	repository := &paymentRepository{claim: true, invoice: Invoice{ID: "invoice-1", TenantID: "tenant-1", ApplicationID: "app-1", Currency: "CNY", TotalMinor: 900, Status: "open", Audit: Audit{Version: 1}}}
	gateway := &paymentGatewayStub{result: PaymentGatewayResult{ProviderPaymentID: "provider-payment-1", ProviderEventID: "provider-event-1", Status: "succeeded", ProcessedAt: now}}
	service := newTestService(t, repository, nil)
	service.transactor, service.gateway, service.now = transactionStub{}, gateway, func() time.Time { return now }
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	payment, duplicate, err := service.CreatePaymentAttempt(ctx, "tenant-1", "app-1", "invoice-1", "demo", "payment-method-secret", "payment-key")
	if err != nil {
		t.Fatal(err)
	}
	if duplicate || payment.Status != "succeeded" || gateway.command.PaymentMethodReference != "payment-method-secret" || gateway.command.AmountMinor != 900 {
		t.Fatalf("payment=%+v duplicate=%v command=%+v", payment, duplicate, gateway.command)
	}
}

func TestImportPlanUsesCodeAsReplayBoundary(t *testing.T) {
	t.Parallel()
	repository := &planImportRepository{}
	service := newTestService(t, repository, nil)
	service.transactor = transactionStub{}
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "import-service", Type: platformprincipal.TypeServiceAccount})
	input := Plan{Code: " pro-monthly ", Name: " Pro ", Currency: "cny", BillingInterval: "MONTH", BaseAmountMinor: 9900, EntitlementsJSON: `{ "seats": 10, "features": ["reports"] }`}
	created, duplicate, err := service.ImportPlan(ctx, input)
	if err != nil || duplicate || created.Code != "pro-monthly" || created.EntitlementsJSON != `{"features":["reports"],"seats":10}` {
		t.Fatalf("created=%+v duplicate=%v err=%v", created, duplicate, err)
	}
	// PostgreSQL JSONB does not preserve the input whitespace representation.
	repository.plan.EntitlementsJSON = `{"features":["reports"],"seats": 10}`
	input.EntitlementsJSON = `{"seats":10,"features":["reports"]}`
	replayed, duplicate, err := service.ImportPlan(ctx, input)
	if err != nil || !duplicate || replayed.ID != created.ID || repository.creates != 1 {
		t.Fatalf("replayed=%+v duplicate=%v creates=%d err=%v", replayed, duplicate, repository.creates, err)
	}
}

func TestCreatePaymentAttemptReplaysCompletedResultAfterInvoicePaid(t *testing.T) {
	t.Parallel()
	repository := &paymentRepository{payment: PaymentAttempt{ID: "payment-1", InvoiceID: "invoice-1", TenantID: "tenant-1", ApplicationID: "app-1", Provider: "demo", IdempotencyKey: "payment-key", RequestHash: hashParts("tenant-1", "app-1", "invoice-1", "demo", "payment-method-secret"), Status: "succeeded"}, invoice: Invoice{ID: "invoice-1", TenantID: "tenant-1", ApplicationID: "app-1", TotalMinor: 900, PaidMinor: 900, Status: "paid"}}
	service := newTestService(t, repository, nil)
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	payment, duplicate, err := service.CreatePaymentAttempt(ctx, "tenant-1", "app-1", "invoice-1", "demo", "payment-method-secret", "payment-key")
	if err != nil || !duplicate || payment.Status != "succeeded" {
		t.Fatalf("payment=%+v duplicate=%v err=%v", payment, duplicate, err)
	}
}

func TestApplyPaymentResultMarksInvoicePaid(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 31, 9, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	repository := &paymentRepository{claim: true, payment: PaymentAttempt{ID: "payment-1", InvoiceID: "invoice-1", TenantID: "tenant-1", ApplicationID: "app-1", Provider: "test", AmountMinor: 1200, Status: "pending", Audit: Audit{Version: 1}}, invoice: Invoice{ID: "invoice-1", TenantID: "tenant-1", ApplicationID: "app-1", TotalMinor: 1200, Status: "open", Audit: Audit{Version: 2}}}
	service := newTestService(t, repository, nil)
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
	repository := &paymentRepository{claim: false, payment: PaymentAttempt{ID: "payment-1", InvoiceID: "invoice-1", TenantID: "tenant-1", ApplicationID: "app-1", Provider: "test", AmountMinor: 100, Status: "succeeded", Audit: Audit{Version: 2}}, invoice: Invoice{ID: "invoice-1", TenantID: "tenant-1", ApplicationID: "app-1", TotalMinor: 100, PaidMinor: 100, Status: "paid", Audit: Audit{Version: 2}}}
	service := newTestService(t, repository, nil)
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
	repository := &paymentRepository{payment: PaymentAttempt{ID: "payment-1", InvoiceID: "invoice-1", TenantID: "tenant-1", ApplicationID: "app-1", Status: "succeeded"}, invoice: Invoice{ID: "invoice-1", TenantID: "tenant-1", ApplicationID: "app-1", PaidMinor: 1000, RefundedMinor: 800}}
	service := newTestService(t, repository, nil)
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	if _, _, _, err := service.RecordRefund(ctx, "tenant-1", "app-1", "payment-1", "provider-refund", "refund-key", 201, "duplicate", "succeeded"); err == nil {
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

func (r *previewRepository) GetSubscription(context.Context, string, string, string) (Subscription, error) {
	return r.subscription, nil
}
func (r *previewRepository) GetPlan(context.Context, string, string) (Plan, error) {
	return r.plan, nil
}
func (r *previewRepository) ListUsagePrices(context.Context, string) ([]UsagePrice, error) {
	return r.prices, nil
}

type usageStub struct {
	quantities    map[string]int64
	applicationID *string
}

func (u usageStub) Total(_ context.Context, _, applicationID, meter string, _, _ time.Time) (int64, error) {
	if u.applicationID != nil {
		*u.applicationID = applicationID
	}
	return u.quantities[meter], nil
}

func TestPreviewInvoiceCalculatesBaseAndMeteredUsage(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	repository := &previewRepository{
		subscription: Subscription{ID: "subscription-1", TenantID: "tenant-1", ApplicationID: "app-1", PlanID: "plan-1", CurrentPeriodStart: start, CurrentPeriodEnd: start.AddDate(0, 1, 0)},
		plan:         Plan{ID: "plan-1", Name: "Pro", Currency: "CNY", BaseAmountMinor: 1000},
		prices:       []UsagePrice{{MeterCode: "api.calls", IncludedQuantity: 100, UnitQuantity: 10, UnitAmountMinor: 25}},
	}
	gotApplicationID := ""
	service := newTestService(t, repository, usageStub{quantities: map[string]int64{"api.calls": 126}, applicationID: &gotApplicationID})
	service.now = func() time.Time { return start }
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	preview, err := service.PreviewInvoice(ctx, "tenant-1", "app-1", "subscription-1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Invoice.TotalMinor != 1075 || len(preview.Lines) != 2 || preview.Lines[1].Quantity != 126 || preview.Lines[1].AmountMinor != 75 {
		t.Fatalf("unexpected preview: %+v lines=%+v", preview.Invoice, preview.Lines)
	}
	if preview.Invoice.ApplicationID != "app-1" || gotApplicationID != "app-1" {
		t.Fatalf("application scope invoice=%q metering=%q", preview.Invoice.ApplicationID, gotApplicationID)
	}
}
func TestPreviewInvoiceRejectsCrossTenant(t *testing.T) {
	t.Parallel()
	service := newTestService(t, &previewRepository{}, usageStub{})
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	if _, err := service.PreviewInvoice(ctx, "tenant-2", "app-1", "subscription-1", time.Now(), time.Now().Add(time.Hour)); err == nil {
		t.Fatal("expected tenant access denial")
	}
}

func TestPreviewInvoiceRejectsApplicationWithoutGrant(t *testing.T) {
	t.Parallel()
	service, err := NewService(&previewRepository{}, nil, usageStub{}, rejectingApplicationVerifier{err: appaccess.ErrNotGranted})
	if err != nil {
		t.Fatal(err)
	}
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	if _, err := service.PreviewInvoice(ctx, "tenant-1", "app-denied", "subscription-1", time.Now(), time.Now().Add(time.Hour)); err == nil {
		t.Fatal("expected application access denial")
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
