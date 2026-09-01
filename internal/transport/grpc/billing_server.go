package grpctransport

import (
	"context"
	"errors"
	"time"

	"github.com/lihongjie0209/billing-service/internal/apperror"
	"github.com/lihongjie0209/billing-service/internal/billing"
	billingv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/billing/v1"
	commonv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type billingServer struct {
	billingv1.UnimplementedBillingServiceServer
	service *billing.Service
}

func (s *billingServer) CreatePlan(ctx context.Context, r *billingv1.CreatePlanRequest) (*billingv1.CreatePlanResponse, error) {
	v, err := s.service.CreatePlan(ctx, billing.Plan{Code: r.GetCode(), Name: r.GetName(), Description: r.GetDescription(), Currency: r.GetCurrency(), BillingInterval: r.GetBillingInterval(), BaseAmountMinor: r.GetBaseAmountMinor(), TrialDays: r.GetTrialDays(), EntitlementsJSON: r.GetEntitlementsJson()})
	return &billingv1.CreatePlanResponse{Plan: billing.ToProtoPlan(v)}, billingError(err)
}
func (s *billingServer) UpdatePlan(ctx context.Context, r *billingv1.UpdatePlanRequest) (*billingv1.UpdatePlanResponse, error) {
	v, err := s.service.UpdatePlan(ctx, billing.Plan{ID: r.GetId(), Name: r.GetName(), Description: r.GetDescription(), BaseAmountMinor: r.GetBaseAmountMinor(), TrialDays: r.GetTrialDays(), Status: r.GetStatus(), EntitlementsJSON: r.GetEntitlementsJson()}, r.GetVersion())
	return &billingv1.UpdatePlanResponse{Plan: billing.ToProtoPlan(v)}, billingError(err)
}
func (s *billingServer) GetPlan(ctx context.Context, r *billingv1.GetPlanRequest) (*billingv1.GetPlanResponse, error) {
	v, prices, err := s.service.GetPlan(ctx, r.GetId(), r.GetCode())
	return &billingv1.GetPlanResponse{Plan: billing.ToProtoPlan(v), UsagePrices: protoPrices(prices)}, billingError(err)
}
func (s *billingServer) ListPlans(ctx context.Context, r *billingv1.ListPlansRequest) (*billingv1.ListPlansResponse, error) {
	page, size := pageValues(r.GetPage())
	values, err := s.service.ListPlans(ctx, r.GetStatus(), r.GetKeyword(), page, size)
	items := make([]*billingv1.Plan, len(values.Items))
	for i := range values.Items {
		items[i] = billing.ToProtoPlan(values.Items[i])
	}
	return &billingv1.ListPlansResponse{Plans: items, Page: pageResult(values.Total, values.Page, values.PageSize)}, billingError(err)
}
func (s *billingServer) UpsertUsagePrice(ctx context.Context, r *billingv1.UpsertUsagePriceRequest) (*billingv1.UpsertUsagePriceResponse, error) {
	v, err := s.service.UpsertUsagePrice(ctx, billing.UsagePrice{ID: r.GetId(), PlanID: r.GetPlanId(), MeterCode: r.GetMeterCode(), IncludedQuantity: r.GetIncludedQuantity(), UnitQuantity: r.GetUnitQuantity(), UnitAmountMinor: r.GetUnitAmountMinor(), PricingModel: r.GetPricingModel(), TiersJSON: r.GetTiersJson()}, r.GetExpectedVersion())
	return &billingv1.UpsertUsagePriceResponse{UsagePrice: billing.ToProtoUsagePrice(v)}, billingError(err)
}
func (s *billingServer) DeleteUsagePrice(ctx context.Context, r *billingv1.DeleteUsagePriceRequest) (*billingv1.DeleteUsagePriceResponse, error) {
	err := s.service.DeleteUsagePrice(ctx, r.GetId(), r.GetVersion())
	return &billingv1.DeleteUsagePriceResponse{}, billingError(err)
}
func (s *billingServer) CreateSubscription(ctx context.Context, r *billingv1.CreateSubscriptionRequest) (*billingv1.CreateSubscriptionResponse, error) {
	v, err := s.service.CreateSubscription(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetPlanId(), timestampValue(r.GetStartsAt()), r.GetExternalReference())
	return &billingv1.CreateSubscriptionResponse{Subscription: billing.ToProtoSubscription(v)}, billingError(err)
}
func (s *billingServer) ChangeSubscription(ctx context.Context, r *billingv1.ChangeSubscriptionRequest) (*billingv1.ChangeSubscriptionResponse, error) {
	v, err := s.service.ChangeSubscription(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetId(), r.GetPlanId(), r.GetEffectiveMode(), r.GetVersion())
	return &billingv1.ChangeSubscriptionResponse{Subscription: billing.ToProtoSubscription(v)}, billingError(err)
}
func (s *billingServer) CancelSubscription(ctx context.Context, r *billingv1.CancelSubscriptionRequest) (*billingv1.CancelSubscriptionResponse, error) {
	v, err := s.service.CancelSubscription(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetId(), r.GetAtPeriodEnd(), r.GetVersion())
	return &billingv1.CancelSubscriptionResponse{Subscription: billing.ToProtoSubscription(v)}, billingError(err)
}
func (s *billingServer) GetSubscription(ctx context.Context, r *billingv1.GetSubscriptionRequest) (*billingv1.GetSubscriptionResponse, error) {
	v, plan, err := s.service.GetSubscription(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetId())
	return &billingv1.GetSubscriptionResponse{Subscription: billing.ToProtoSubscription(v), Plan: billing.ToProtoPlan(plan)}, billingError(err)
}
func (s *billingServer) ListSubscriptions(ctx context.Context, r *billingv1.ListSubscriptionsRequest) (*billingv1.ListSubscriptionsResponse, error) {
	page, size := pageValues(r.GetPage())
	values, err := s.service.ListSubscriptions(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetStatus(), page, size)
	items := make([]*billingv1.Subscription, len(values.Items))
	for i := range values.Items {
		items[i] = billing.ToProtoSubscription(values.Items[i])
	}
	return &billingv1.ListSubscriptionsResponse{Subscriptions: items, Page: pageResult(values.Total, values.Page, values.PageSize)}, billingError(err)
}
func (s *billingServer) PreviewInvoice(ctx context.Context, r *billingv1.PreviewInvoiceRequest) (*billingv1.PreviewInvoiceResponse, error) {
	v, err := s.service.PreviewInvoice(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetSubscriptionId(), timestampValue(r.GetPeriodStart()), timestampValue(r.GetPeriodEnd()))
	return &billingv1.PreviewInvoiceResponse{Invoice: billing.ToProtoInvoice(v.Invoice), Lines: billing.ToProtoInvoiceLines(v.Lines)}, billingError(err)
}
func (s *billingServer) GenerateInvoice(ctx context.Context, r *billingv1.GenerateInvoiceRequest) (*billingv1.GenerateInvoiceResponse, error) {
	v, duplicate, err := s.service.GenerateInvoice(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetSubscriptionId(), timestampValue(r.GetPeriodStart()), timestampValue(r.GetPeriodEnd()), r.GetIdempotencyKey())
	return &billingv1.GenerateInvoiceResponse{Invoice: billing.ToProtoInvoice(v.Invoice), Lines: billing.ToProtoInvoiceLines(v.Lines), Duplicate: duplicate}, billingError(err)
}
func (s *billingServer) FinalizeInvoice(ctx context.Context, r *billingv1.FinalizeInvoiceRequest) (*billingv1.FinalizeInvoiceResponse, error) {
	v, err := s.service.FinalizeInvoice(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetId(), timestampValue(r.GetDueAt()), r.GetVersion())
	return &billingv1.FinalizeInvoiceResponse{Invoice: billing.ToProtoInvoice(v)}, billingError(err)
}
func (s *billingServer) VoidInvoice(ctx context.Context, r *billingv1.VoidInvoiceRequest) (*billingv1.VoidInvoiceResponse, error) {
	v, err := s.service.VoidInvoice(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetId(), r.GetReason(), r.GetVersion())
	return &billingv1.VoidInvoiceResponse{Invoice: billing.ToProtoInvoice(v)}, billingError(err)
}
func (s *billingServer) GetInvoice(ctx context.Context, r *billingv1.GetInvoiceRequest) (*billingv1.GetInvoiceResponse, error) {
	v, lines, err := s.service.GetInvoice(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetId())
	return &billingv1.GetInvoiceResponse{Invoice: billing.ToProtoInvoice(v), Lines: billing.ToProtoInvoiceLines(lines)}, billingError(err)
}
func (s *billingServer) ListInvoices(ctx context.Context, r *billingv1.ListInvoicesRequest) (*billingv1.ListInvoicesResponse, error) {
	page, size := pageValues(r.GetPage())
	values, err := s.service.ListInvoices(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetStatus(), timestampValue(r.GetCreatedFrom()), timestampValue(r.GetCreatedTo()), page, size)
	items := make([]*billingv1.Invoice, len(values.Items))
	for i := range values.Items {
		items[i] = billing.ToProtoInvoice(values.Items[i])
	}
	return &billingv1.ListInvoicesResponse{Invoices: items, Page: pageResult(values.Total, values.Page, values.PageSize)}, billingError(err)
}
func (s *billingServer) CreatePaymentAttempt(ctx context.Context, r *billingv1.CreatePaymentAttemptRequest) (*billingv1.CreatePaymentAttemptResponse, error) {
	v, duplicate, err := s.service.CreatePaymentAttempt(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetInvoiceId(), r.GetProvider(), r.GetPaymentMethodReference(), r.GetIdempotencyKey())
	return &billingv1.CreatePaymentAttemptResponse{PaymentAttempt: billing.ToProtoPayment(v), Duplicate: duplicate}, billingError(err)
}
func (s *billingServer) ApplyPaymentResult(ctx context.Context, r *billingv1.ApplyPaymentResultRequest) (*billingv1.ApplyPaymentResultResponse, error) {
	payment, invoice, duplicate, err := s.service.ApplyPaymentResult(ctx, r.GetPaymentAttemptId(), r.GetProviderPaymentId(), r.GetProviderEventId(), r.GetStatus(), r.GetFailureCode(), r.GetFailureMessage(), timestampValue(r.GetProcessedAt()))
	return &billingv1.ApplyPaymentResultResponse{PaymentAttempt: billing.ToProtoPayment(payment), Invoice: billing.ToProtoInvoice(invoice), Duplicate: duplicate}, billingError(err)
}
func (s *billingServer) RecordRefund(ctx context.Context, r *billingv1.RecordRefundRequest) (*billingv1.RecordRefundResponse, error) {
	refund, invoice, duplicate, err := s.service.RecordRefund(ctx, r.GetTenantId(), r.GetApplicationId(), r.GetPaymentAttemptId(), r.GetProviderRefundId(), r.GetIdempotencyKey(), r.GetAmountMinor(), r.GetReason(), r.GetStatus())
	return &billingv1.RecordRefundResponse{Refund: billing.ToProtoRefund(refund), Invoice: billing.ToProtoInvoice(invoice), Duplicate: duplicate}, billingError(err)
}
func (s *billingServer) ReconcilePayment(ctx context.Context, r *billingv1.ReconcilePaymentRequest) (*billingv1.ReconcilePaymentResponse, error) {
	values, next, err := s.service.ReconcilePayment(ctx, r.GetProvider(), timestampValue(r.GetFrom()), timestampValue(r.GetTo()), r.GetCursor(), r.GetLimit())
	items := make([]*billingv1.ReconciliationMismatch, len(values))
	for i, value := range values {
		items[i] = &billingv1.ReconciliationMismatch{ProviderPaymentId: value.ProviderPaymentID, PaymentAttemptId: value.PaymentAttemptID, Reason: value.Reason, ProviderAmountMinor: value.ProviderAmountMinor, LocalAmountMinor: value.LocalAmountMinor}
	}
	return &billingv1.ReconcilePaymentResponse{Mismatches: items, NextCursor: next}, billingError(err)
}

func protoPrices(values []billing.UsagePrice) []*billingv1.UsagePrice {
	items := make([]*billingv1.UsagePrice, len(values))
	for i := range values {
		items[i] = billing.ToProtoUsagePrice(values[i])
	}
	return items
}
func pageValues(v *commonv1.PageRequest) (int, int) {
	if v == nil {
		return 0, 0
	}
	return int(v.GetPage()), int(v.GetPageSize())
}
func pageResult(total int64, page, size int) *commonv1.PageResult {
	return &commonv1.PageResult{Total: uint64(total), Page: uint32(page), PageSize: uint32(size)}
}
func timestampValue(v *timestamppb.Timestamp) time.Time {
	if v == nil || !v.IsValid() {
		return time.Time{}
	}
	return v.AsTime()
}
func billingError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return status.Error(codes.Internal, "internal server error")
	}
	codesByApp := map[int]codes.Code{apperror.CodeInvalidArgument: codes.InvalidArgument, apperror.CodeNotFound: codes.NotFound, apperror.CodeUnauthorized: codes.Unauthenticated, apperror.CodeForbidden: codes.PermissionDenied, apperror.CodeConflict: codes.Aborted, apperror.CodeRequestInProgress: codes.Aborted, apperror.CodeDependencyUnavailable: codes.Unavailable, apperror.CodeRequestTimeout: codes.DeadlineExceeded, apperror.CodeTooManyRequests: codes.ResourceExhausted}
	code, ok := codesByApp[appErr.Code]
	if !ok {
		code = codes.Internal
	}
	return status.Error(code, appErr.Message)
}
