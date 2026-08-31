package billing

import "time"

type Audit struct {
	Version   int64     `db:"version" json:"version"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	CreatedBy string    `db:"created_by" json:"created_by"`
	UpdatedBy string    `db:"updated_by" json:"updated_by"`
}

type Plan struct {
	ID               string `db:"id" json:"id"`
	Code             string `db:"code" json:"code"`
	Name             string `db:"name" json:"name"`
	Description      string `db:"description" json:"description"`
	Currency         string `db:"currency" json:"currency"`
	BillingInterval  string `db:"billing_interval" json:"billing_interval"`
	BaseAmountMinor  int64  `db:"base_amount_minor" json:"base_amount_minor"`
	TrialDays        int32  `db:"trial_days" json:"trial_days"`
	Status           string `db:"status" json:"status"`
	EntitlementsJSON string `db:"entitlements_json" json:"entitlements_json"`
	Audit
}

type UsagePrice struct {
	ID               string `db:"id" json:"id"`
	PlanID           string `db:"plan_id" json:"plan_id"`
	MeterCode        string `db:"meter_code" json:"meter_code"`
	IncludedQuantity int64  `db:"included_quantity" json:"included_quantity"`
	UnitQuantity     int64  `db:"unit_quantity" json:"unit_quantity"`
	UnitAmountMinor  int64  `db:"unit_amount_minor" json:"unit_amount_minor"`
	PricingModel     string `db:"pricing_model" json:"pricing_model"`
	TiersJSON        string `db:"tiers_json" json:"tiers_json"`
	Audit
}

type Subscription struct {
	ID                 string     `db:"id" json:"id"`
	TenantID           string     `db:"tenant_id" json:"tenant_id"`
	PlanID             string     `db:"plan_id" json:"plan_id"`
	Status             string     `db:"status" json:"status"`
	CurrentPeriodStart time.Time  `db:"current_period_start" json:"current_period_start"`
	CurrentPeriodEnd   time.Time  `db:"current_period_end" json:"current_period_end"`
	CancelAtPeriodEnd  bool       `db:"cancel_at_period_end" json:"cancel_at_period_end"`
	CanceledAt         *time.Time `db:"canceled_at" json:"canceled_at,omitempty"`
	ExternalReference  string     `db:"external_reference" json:"external_reference"`
	PendingPlanID      string     `db:"pending_plan_id" json:"pending_plan_id"`
	PendingChangeAt    *time.Time `db:"pending_change_at" json:"pending_change_at,omitempty"`
	Audit
}

type Invoice struct {
	ID             string     `db:"id" json:"id"`
	Number         string     `db:"number" json:"number"`
	TenantID       string     `db:"tenant_id" json:"tenant_id"`
	SubscriptionID string     `db:"subscription_id" json:"subscription_id"`
	Currency       string     `db:"currency" json:"currency"`
	Status         string     `db:"status" json:"status"`
	PeriodStart    time.Time  `db:"period_start" json:"period_start"`
	PeriodEnd      time.Time  `db:"period_end" json:"period_end"`
	SubtotalMinor  int64      `db:"subtotal_minor" json:"subtotal_minor"`
	DiscountMinor  int64      `db:"discount_minor" json:"discount_minor"`
	TaxMinor       int64      `db:"tax_minor" json:"tax_minor"`
	TotalMinor     int64      `db:"total_minor" json:"total_minor"`
	PaidMinor      int64      `db:"paid_minor" json:"paid_minor"`
	RefundedMinor  int64      `db:"refunded_minor" json:"refunded_minor"`
	DueAt          *time.Time `db:"due_at" json:"due_at,omitempty"`
	FinalizedAt    *time.Time `db:"finalized_at" json:"finalized_at,omitempty"`
	PaidAt         *time.Time `db:"paid_at" json:"paid_at,omitempty"`
	Audit
}

type InvoiceLine struct {
	ID              string `db:"id" json:"id"`
	InvoiceID       string `db:"invoice_id" json:"invoice_id"`
	Type            string `db:"type" json:"type"`
	Description     string `db:"description" json:"description"`
	MeterCode       string `db:"meter_code" json:"meter_code"`
	Quantity        int64  `db:"quantity" json:"quantity"`
	UnitQuantity    int64  `db:"unit_quantity" json:"unit_quantity"`
	UnitAmountMinor int64  `db:"unit_amount_minor" json:"unit_amount_minor"`
	AmountMinor     int64  `db:"amount_minor" json:"amount_minor"`
	MetadataJSON    string `db:"metadata_json" json:"metadata_json"`
	Audit
}

type PaymentAttempt struct {
	ID                string     `db:"id" json:"id"`
	InvoiceID         string     `db:"invoice_id" json:"invoice_id"`
	TenantID          string     `db:"tenant_id" json:"tenant_id"`
	Provider          string     `db:"provider" json:"provider"`
	ProviderPaymentID string     `db:"provider_payment_id" json:"provider_payment_id"`
	IdempotencyKey    string     `db:"idempotency_key" json:"idempotency_key"`
	RequestHash       string     `db:"request_hash" json:"-"`
	Currency          string     `db:"currency" json:"currency"`
	AmountMinor       int64      `db:"amount_minor" json:"amount_minor"`
	Status            string     `db:"status" json:"status"`
	FailureCode       string     `db:"failure_code" json:"failure_code"`
	FailureMessage    string     `db:"failure_message" json:"failure_message"`
	ProcessedAt       *time.Time `db:"processed_at" json:"processed_at,omitempty"`
	Audit
}

type Refund struct {
	ID               string `db:"id" json:"id"`
	PaymentAttemptID string `db:"payment_attempt_id" json:"payment_attempt_id"`
	InvoiceID        string `db:"invoice_id" json:"invoice_id"`
	ProviderRefundID string `db:"provider_refund_id" json:"provider_refund_id"`
	IdempotencyKey   string `db:"idempotency_key" json:"idempotency_key"`
	RequestHash      string `db:"request_hash" json:"-"`
	AmountMinor      int64  `db:"amount_minor" json:"amount_minor"`
	Reason           string `db:"reason" json:"reason"`
	Status           string `db:"status" json:"status"`
	Audit
}

type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

type InvoicePreview struct {
	Invoice Invoice       `json:"invoice"`
	Lines   []InvoiceLine `json:"lines"`
}
