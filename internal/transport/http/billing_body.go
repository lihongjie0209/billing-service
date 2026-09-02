package httptransport

import (
	"encoding/json"
	"time"

	"github.com/lihongjie0209/billing-service/internal/billing"
)

type PlanBody struct {
	ID               string          `json:"id"`
	Code             string          `json:"code"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Currency         string          `json:"currency"`
	BillingInterval  string          `json:"billing_interval"`
	BaseAmountMinor  int64           `json:"base_amount_minor"`
	TrialDays        int32           `json:"trial_days"`
	Status           string          `json:"status"`
	EntitlementsJSON json.RawMessage `json:"entitlements_json" swaggertype:"object"`
	Version          int64           `json:"version"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	CreatedBy        string          `json:"created_by"`
	UpdatedBy        string          `json:"updated_by"`
}
type UsagePriceBody struct {
	ID               string          `json:"id"`
	PlanID           string          `json:"plan_id"`
	MeterCode        string          `json:"meter_code"`
	IncludedQuantity int64           `json:"included_quantity"`
	UnitQuantity     int64           `json:"unit_quantity"`
	UnitAmountMinor  int64           `json:"unit_amount_minor"`
	PricingModel     string          `json:"pricing_model"`
	TiersJSON        json.RawMessage `json:"tiers_json" swaggertype:"array,object"`
	Version          int64           `json:"version"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	CreatedBy        string          `json:"created_by"`
	UpdatedBy        string          `json:"updated_by"`
}
type SubscriptionBody struct {
	ID                 string     `json:"id"`
	TenantID           string     `json:"tenant_id"`
	ApplicationID      string     `json:"application_id"`
	PlanID             string     `json:"plan_id"`
	Status             string     `json:"status"`
	CurrentPeriodStart time.Time  `json:"current_period_start"`
	CurrentPeriodEnd   time.Time  `json:"current_period_end"`
	CancelAtPeriodEnd  bool       `json:"cancel_at_period_end"`
	CanceledAt         *time.Time `json:"canceled_at,omitempty"`
	ExternalReference  string     `json:"external_reference"`
	PendingPlanID      *string    `json:"pending_plan_id,omitempty"`
	PendingChangeAt    *time.Time `json:"pending_change_at,omitempty"`
	Version            int64      `json:"version"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	CreatedBy          string     `json:"created_by"`
	UpdatedBy          string     `json:"updated_by"`
}
type InvoiceBody struct {
	ID             string     `json:"id"`
	Number         string     `json:"number"`
	TenantID       string     `json:"tenant_id"`
	ApplicationID  string     `json:"application_id"`
	SubscriptionID string     `json:"subscription_id"`
	Currency       string     `json:"currency"`
	Status         string     `json:"status"`
	PeriodStart    time.Time  `json:"period_start"`
	PeriodEnd      time.Time  `json:"period_end"`
	SubtotalMinor  int64      `json:"subtotal_minor"`
	DiscountMinor  int64      `json:"discount_minor"`
	TaxMinor       int64      `json:"tax_minor"`
	TotalMinor     int64      `json:"total_minor"`
	PaidMinor      int64      `json:"paid_minor"`
	RefundedMinor  int64      `json:"refunded_minor"`
	DueAt          *time.Time `json:"due_at,omitempty"`
	FinalizedAt    *time.Time `json:"finalized_at,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	Version        int64      `json:"version"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CreatedBy      string     `json:"created_by"`
	UpdatedBy      string     `json:"updated_by"`
}
type InvoiceLineBody struct {
	ID              string          `json:"id"`
	InvoiceID       string          `json:"invoice_id"`
	Type            string          `json:"type"`
	Description     string          `json:"description"`
	MeterCode       string          `json:"meter_code"`
	Quantity        int64           `json:"quantity"`
	UnitQuantity    int64           `json:"unit_quantity"`
	UnitAmountMinor int64           `json:"unit_amount_minor"`
	AmountMinor     int64           `json:"amount_minor"`
	MetadataJSON    json.RawMessage `json:"metadata_json" swaggertype:"object"`
	Version         int64           `json:"version"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	CreatedBy       string          `json:"created_by"`
	UpdatedBy       string          `json:"updated_by"`
}
type PaymentAttemptBody struct {
	ID                string     `json:"id"`
	InvoiceID         string     `json:"invoice_id"`
	TenantID          string     `json:"tenant_id"`
	ApplicationID     string     `json:"application_id"`
	Provider          string     `json:"provider"`
	ProviderPaymentID string     `json:"provider_payment_id"`
	Currency          string     `json:"currency"`
	AmountMinor       int64      `json:"amount_minor"`
	Status            string     `json:"status"`
	FailureCode       string     `json:"failure_code"`
	FailureMessage    string     `json:"failure_message"`
	ProcessedAt       *time.Time `json:"processed_at,omitempty"`
	Version           int64      `json:"version"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	CreatedBy         string     `json:"created_by"`
	UpdatedBy         string     `json:"updated_by"`
}
type RefundBody struct {
	ID               string    `json:"id"`
	PaymentAttemptID string    `json:"payment_attempt_id"`
	InvoiceID        string    `json:"invoice_id"`
	TenantID         string    `json:"tenant_id"`
	ApplicationID    string    `json:"application_id"`
	ProviderRefundID string    `json:"provider_refund_id"`
	AmountMinor      int64     `json:"amount_minor"`
	Reason           string    `json:"reason"`
	Status           string    `json:"status"`
	Version          int64     `json:"version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedBy        string    `json:"created_by"`
	UpdatedBy        string    `json:"updated_by"`
}

type PlanPageBody struct {
	Items    []PlanBody `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}
type SubscriptionPageBody struct {
	Items    []SubscriptionBody `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}
type InvoicePageBody struct {
	Items    []InvoiceBody `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}
type PaymentAttemptPageBody struct {
	Items    []PaymentAttemptBody `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}
type RefundPageBody struct {
	Items    []RefundBody `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}
type PlanDetailBody struct {
	Plan        PlanBody         `json:"plan"`
	UsagePrices []UsagePriceBody `json:"usage_prices"`
}
type SubscriptionDetailBody struct {
	Subscription SubscriptionBody `json:"subscription"`
	Plan         PlanBody         `json:"plan"`
}
type InvoiceDetailBody struct {
	Invoice InvoiceBody       `json:"invoice"`
	Lines   []InvoiceLineBody `json:"lines"`
}
type GenerateInvoiceBody struct {
	Invoice   InvoiceBody       `json:"invoice"`
	Lines     []InvoiceLineBody `json:"lines"`
	Duplicate bool              `json:"duplicate"`
}
type CreatePaymentAttemptBody struct {
	PaymentAttempt PaymentAttemptBody `json:"payment_attempt"`
	Duplicate      bool               `json:"duplicate"`
}
type ApplyPaymentResultBody struct {
	PaymentAttempt PaymentAttemptBody `json:"payment_attempt"`
	Invoice        InvoiceBody        `json:"invoice"`
	Duplicate      bool               `json:"duplicate"`
}
type RecordRefundBody struct {
	Refund    RefundBody  `json:"refund"`
	Invoice   InvoiceBody `json:"invoice"`
	Duplicate bool        `json:"duplicate"`
}

func planBody(v billing.Plan) PlanBody {
	return PlanBody{ID: v.ID, Code: v.Code, Name: v.Name, Description: v.Description, Currency: v.Currency, BillingInterval: v.BillingInterval, BaseAmountMinor: v.BaseAmountMinor, TrialDays: v.TrialDays, Status: v.Status, EntitlementsJSON: rawObject(json.RawMessage(v.EntitlementsJSON)), Version: v.Version, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func usagePriceBody(v billing.UsagePrice) UsagePriceBody {
	return UsagePriceBody{ID: v.ID, PlanID: v.PlanID, MeterCode: v.MeterCode, IncludedQuantity: v.IncludedQuantity, UnitQuantity: v.UnitQuantity, UnitAmountMinor: v.UnitAmountMinor, PricingModel: v.PricingModel, TiersJSON: rawArray(json.RawMessage(v.TiersJSON)), Version: v.Version, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func subscriptionBody(v billing.Subscription) SubscriptionBody {
	return SubscriptionBody{ID: v.ID, TenantID: v.TenantID, ApplicationID: v.ApplicationID, PlanID: v.PlanID, Status: v.Status, CurrentPeriodStart: v.CurrentPeriodStart, CurrentPeriodEnd: v.CurrentPeriodEnd, CancelAtPeriodEnd: v.CancelAtPeriodEnd, CanceledAt: v.CanceledAt, ExternalReference: v.ExternalReference, PendingPlanID: v.PendingPlanID, PendingChangeAt: v.PendingChangeAt, Version: v.Version, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func invoiceBody(v billing.Invoice) InvoiceBody {
	return InvoiceBody{ID: v.ID, Number: v.Number, TenantID: v.TenantID, ApplicationID: v.ApplicationID, SubscriptionID: v.SubscriptionID, Currency: v.Currency, Status: v.Status, PeriodStart: v.PeriodStart, PeriodEnd: v.PeriodEnd, SubtotalMinor: v.SubtotalMinor, DiscountMinor: v.DiscountMinor, TaxMinor: v.TaxMinor, TotalMinor: v.TotalMinor, PaidMinor: v.PaidMinor, RefundedMinor: v.RefundedMinor, DueAt: v.DueAt, FinalizedAt: v.FinalizedAt, PaidAt: v.PaidAt, Version: v.Version, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func invoiceLineBody(v billing.InvoiceLine) InvoiceLineBody {
	return InvoiceLineBody{ID: v.ID, InvoiceID: v.InvoiceID, Type: v.Type, Description: v.Description, MeterCode: v.MeterCode, Quantity: v.Quantity, UnitQuantity: v.UnitQuantity, UnitAmountMinor: v.UnitAmountMinor, AmountMinor: v.AmountMinor, MetadataJSON: rawObject(json.RawMessage(v.MetadataJSON)), Version: v.Version, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func paymentAttemptBody(v billing.PaymentAttempt) PaymentAttemptBody {
	return PaymentAttemptBody{ID: v.ID, InvoiceID: v.InvoiceID, TenantID: v.TenantID, ApplicationID: v.ApplicationID, Provider: v.Provider, ProviderPaymentID: v.ProviderPaymentID, Currency: v.Currency, AmountMinor: v.AmountMinor, Status: v.Status, FailureCode: v.FailureCode, FailureMessage: v.FailureMessage, ProcessedAt: v.ProcessedAt, Version: v.Version, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}
func refundBody(v billing.Refund) RefundBody {
	return RefundBody{ID: v.ID, PaymentAttemptID: v.PaymentAttemptID, InvoiceID: v.InvoiceID, TenantID: v.TenantID, ApplicationID: v.ApplicationID, ProviderRefundID: v.ProviderRefundID, AmountMinor: v.AmountMinor, Reason: v.Reason, Status: v.Status, Version: v.Version, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, CreatedBy: v.CreatedBy, UpdatedBy: v.UpdatedBy}
}

func mapBodies[T, U any](values []T, mapOne func(T) U) []U {
	result := make([]U, len(values))
	for i, value := range values {
		result[i] = mapOne(value)
	}
	return result
}
func planPage(v billing.Page[billing.Plan]) PlanPageBody {
	return PlanPageBody{Items: mapBodies(v.Items, planBody), Total: v.Total, Page: v.Page, PageSize: v.PageSize}
}
func subscriptionPage(v billing.Page[billing.Subscription]) SubscriptionPageBody {
	return SubscriptionPageBody{Items: mapBodies(v.Items, subscriptionBody), Total: v.Total, Page: v.Page, PageSize: v.PageSize}
}
func invoicePage(v billing.Page[billing.Invoice]) InvoicePageBody {
	return InvoicePageBody{Items: mapBodies(v.Items, invoiceBody), Total: v.Total, Page: v.Page, PageSize: v.PageSize}
}
func paymentAttemptPage(v billing.Page[billing.PaymentAttempt]) PaymentAttemptPageBody {
	return PaymentAttemptPageBody{Items: mapBodies(v.Items, paymentAttemptBody), Total: v.Total, Page: v.Page, PageSize: v.PageSize}
}
func refundPage(v billing.Page[billing.Refund]) RefundPageBody {
	return RefundPageBody{Items: mapBodies(v.Items, refundBody), Total: v.Total, Page: v.Page, PageSize: v.PageSize}
}
