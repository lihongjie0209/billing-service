package billing

import (
	"time"

	billingv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/billing/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ToProtoPlan(v Plan) *billingv1.Plan {
	return &billingv1.Plan{Id: v.ID, Code: v.Code, Name: v.Name, Description: v.Description, Currency: v.Currency, BillingInterval: v.BillingInterval, BaseAmountMinor: v.BaseAmountMinor, TrialDays: v.TrialDays, Status: v.Status, EntitlementsJson: v.EntitlementsJSON, Version: v.Version, CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt), CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func ToProtoUsagePrice(v UsagePrice) *billingv1.UsagePrice {
	return &billingv1.UsagePrice{Id: v.ID, PlanId: v.PlanID, MeterCode: v.MeterCode, IncludedQuantity: v.IncludedQuantity, UnitQuantity: v.UnitQuantity, UnitAmountMinor: v.UnitAmountMinor, PricingModel: v.PricingModel, TiersJson: v.TiersJSON, Version: v.Version, CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt), CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func ToProtoSubscription(v Subscription) *billingv1.Subscription {
	return &billingv1.Subscription{Id: v.ID, TenantId: v.TenantID, PlanId: v.PlanID, Status: v.Status, CurrentPeriodStart: timestamppb.New(v.CurrentPeriodStart), CurrentPeriodEnd: timestamppb.New(v.CurrentPeriodEnd), CancelAtPeriodEnd: v.CancelAtPeriodEnd, CanceledAt: nullableTimestamp(v.CanceledAt), ExternalReference: v.ExternalReference, Version: v.Version, CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt), CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy, PendingPlanId: v.PendingPlanID, PendingChangeAt: nullableTimestamp(v.PendingChangeAt)}
}
func ToProtoInvoice(v Invoice) *billingv1.Invoice {
	return &billingv1.Invoice{Id: v.ID, Number: v.Number, TenantId: v.TenantID, SubscriptionId: v.SubscriptionID, Currency: v.Currency, Status: v.Status, PeriodStart: timestamppb.New(v.PeriodStart), PeriodEnd: timestamppb.New(v.PeriodEnd), SubtotalMinor: v.SubtotalMinor, DiscountMinor: v.DiscountMinor, TaxMinor: v.TaxMinor, TotalMinor: v.TotalMinor, PaidMinor: v.PaidMinor, RefundedMinor: v.RefundedMinor, DueAt: nullableTimestamp(v.DueAt), FinalizedAt: nullableTimestamp(v.FinalizedAt), PaidAt: nullableTimestamp(v.PaidAt), Version: v.Version, CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt), CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func ToProtoInvoiceLine(v InvoiceLine) *billingv1.InvoiceLine {
	return &billingv1.InvoiceLine{Id: v.ID, InvoiceId: v.InvoiceID, Type: v.Type, Description: v.Description, MeterCode: v.MeterCode, Quantity: v.Quantity, UnitQuantity: v.UnitQuantity, UnitAmountMinor: v.UnitAmountMinor, AmountMinor: v.AmountMinor, MetadataJson: v.MetadataJSON, Version: v.Version, CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt), CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func ToProtoInvoiceLines(values []InvoiceLine) []*billingv1.InvoiceLine {
	result := make([]*billingv1.InvoiceLine, len(values))
	for i, v := range values {
		result[i] = ToProtoInvoiceLine(v)
	}
	return result
}
func ToProtoPayment(v PaymentAttempt) *billingv1.PaymentAttempt {
	return &billingv1.PaymentAttempt{Id: v.ID, InvoiceId: v.InvoiceID, TenantId: v.TenantID, Provider: v.Provider, ProviderPaymentId: v.ProviderPaymentID, IdempotencyKey: v.IdempotencyKey, Currency: v.Currency, AmountMinor: v.AmountMinor, Status: v.Status, FailureCode: v.FailureCode, FailureMessage: v.FailureMessage, ProcessedAt: nullableTimestamp(v.ProcessedAt), Version: v.Version, CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt), CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func ToProtoRefund(v Refund) *billingv1.Refund {
	return &billingv1.Refund{Id: v.ID, PaymentAttemptId: v.PaymentAttemptID, InvoiceId: v.InvoiceID, ProviderRefundId: v.ProviderRefundID, IdempotencyKey: v.IdempotencyKey, AmountMinor: v.AmountMinor, Reason: v.Reason, Status: v.Status, Version: v.Version, CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt), CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func nullableTimestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamppb.New(*value)
}
