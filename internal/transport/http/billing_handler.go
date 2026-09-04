package httptransport

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lihongjie0209/billing-service/internal/apperror"
	"github.com/lihongjie0209/billing-service/internal/billing"
)

type pageRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}
type idVersionRequest struct {
	TenantID string `json:"tenant_id"`
	ID       string `json:"id"`
	Version  int64  `json:"version"`
}
type createPlanRequest struct {
	Code             string          `json:"code"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Currency         string          `json:"currency"`
	BillingInterval  string          `json:"billing_interval"`
	BaseAmountMinor  int64           `json:"base_amount_minor"`
	TrialDays        int32           `json:"trial_days"`
	EntitlementsJSON json.RawMessage `json:"entitlements_json" swaggertype:"object"`
}
type updatePlanRequest struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	BaseAmountMinor  int64           `json:"base_amount_minor"`
	TrialDays        int32           `json:"trial_days"`
	Status           string          `json:"status"`
	EntitlementsJSON json.RawMessage `json:"entitlements_json" swaggertype:"object"`
	Version          int64           `json:"version"`
}
type getPlanRequest struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}
type listPlansRequest struct {
	Status  string `json:"status"`
	Keyword string `json:"keyword"`
	pageRequest
}
type upsertUsagePriceRequest struct {
	ID               string          `json:"id"`
	PlanID           string          `json:"plan_id"`
	MeterCode        string          `json:"meter_code"`
	IncludedQuantity int64           `json:"included_quantity"`
	UnitQuantity     int64           `json:"unit_quantity"`
	UnitAmountMinor  int64           `json:"unit_amount_minor"`
	PricingModel     string          `json:"pricing_model"`
	TiersJSON        json.RawMessage `json:"tiers_json" swaggertype:"array,object"`
	ExpectedVersion  int64           `json:"expected_version"`
}
type createSubscriptionRequest struct {
	TenantID          string    `json:"tenant_id"`
	ApplicationID     string    `json:"application_id" binding:"required"`
	PlanID            string    `json:"plan_id"`
	StartsAt          time.Time `json:"starts_at"`
	ExternalReference string    `json:"external_reference"`
	PlanVersion       int64     `json:"plan_version"`
}
type changeSubscriptionRequest struct {
	TenantID      string `json:"tenant_id"`
	ApplicationID string `json:"application_id" binding:"required"`
	ID            string `json:"id"`
	PlanID        string `json:"plan_id"`
	EffectiveMode string `json:"effective_mode"`
	Version       int64  `json:"version"`
}
type cancelSubscriptionRequest struct {
	TenantID      string `json:"tenant_id"`
	ApplicationID string `json:"application_id" binding:"required"`
	ID            string `json:"id"`
	AtPeriodEnd   bool   `json:"at_period_end"`
	Version       int64  `json:"version"`
}
type getSubscriptionRequest struct {
	TenantID      string `json:"tenant_id"`
	ApplicationID string `json:"application_id" binding:"required"`
	ID            string `json:"id"`
}
type listSubscriptionsRequest struct {
	TenantID      string `json:"tenant_id"`
	ApplicationID string `json:"application_id" binding:"required"`
	Status        string `json:"status"`
	pageRequest
}
type invoicePeriodRequest struct {
	TenantID       string    `json:"tenant_id"`
	ApplicationID  string    `json:"application_id" binding:"required"`
	SubscriptionID string    `json:"subscription_id"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	IdempotencyKey string    `json:"idempotency_key"`
}
type finalizeInvoiceRequest struct {
	TenantID      string    `json:"tenant_id"`
	ApplicationID string    `json:"application_id" binding:"required"`
	ID            string    `json:"id"`
	DueAt         time.Time `json:"due_at"`
	Version       int64     `json:"version"`
}
type voidInvoiceRequest struct {
	TenantID      string `json:"tenant_id"`
	ApplicationID string `json:"application_id" binding:"required"`
	ID            string `json:"id"`
	Reason        string `json:"reason"`
	Version       int64  `json:"version"`
}
type listInvoicesRequest struct {
	TenantID      string    `json:"tenant_id"`
	ApplicationID string    `json:"application_id" binding:"required"`
	Status        string    `json:"status"`
	CreatedFrom   time.Time `json:"created_from"`
	CreatedTo     time.Time `json:"created_to"`
	pageRequest
}
type getInvoiceRequest struct {
	TenantID      string `json:"tenant_id"`
	ApplicationID string `json:"application_id" binding:"required"`
	ID            string `json:"id"`
}
type createPaymentRequest struct {
	TenantID               string `json:"tenant_id"`
	ApplicationID          string `json:"application_id" binding:"required"`
	InvoiceID              string `json:"invoice_id"`
	Provider               string `json:"provider"`
	PaymentMethodReference string `json:"payment_method_reference"`
	IdempotencyKey         string `json:"idempotency_key"`
}
type applyPaymentRequest struct {
	PaymentAttemptID  string    `json:"payment_attempt_id"`
	ProviderPaymentID string    `json:"provider_payment_id"`
	ProviderEventID   string    `json:"provider_event_id"`
	Status            string    `json:"status"`
	FailureCode       string    `json:"failure_code"`
	FailureMessage    string    `json:"failure_message"`
	ProcessedAt       time.Time `json:"processed_at"`
}
type recordRefundRequest struct {
	TenantID         string `json:"tenant_id"`
	ApplicationID    string `json:"application_id" binding:"required"`
	PaymentAttemptID string `json:"payment_attempt_id"`
	ProviderRefundID string `json:"provider_refund_id"`
	IdempotencyKey   string `json:"idempotency_key"`
	AmountMinor      int64  `json:"amount_minor"`
	Reason           string `json:"reason"`
	Status           string `json:"status"`
}
type listPaymentsRequest struct {
	TenantID      string `json:"tenant_id"`
	ApplicationID string `json:"application_id" binding:"required"`
	Status        string `json:"status"`
	pageRequest
}
type getPaymentRequest struct {
	TenantID      string `json:"tenant_id"`
	ApplicationID string `json:"application_id" binding:"required"`
	ID            string `json:"id" binding:"required"`
}

func registerBillingRoutes(api *gin.RouterGroup, h *Handler) {
	for path, handler := range map[string]gin.HandlerFunc{
		"/plans/create": h.CreatePlan, "/plans/update": h.UpdatePlan, "/plans/get": h.GetPlan, "/plans/list": h.ListPlans,
		"/plans/usage-prices/upsert": h.UpsertUsagePrice, "/plans/usage-prices/delete": h.DeleteUsagePrice,
		"/subscriptions/create": h.CreateSubscription, "/subscriptions/change": h.ChangeSubscription, "/subscriptions/cancel": h.CancelSubscription, "/subscriptions/get": h.GetSubscription, "/subscriptions/list": h.ListSubscriptions,
		"/invoices/preview": h.PreviewInvoice, "/invoices/generate": h.GenerateInvoice, "/invoices/finalize": h.FinalizeInvoice, "/invoices/void": h.VoidInvoice, "/invoices/get": h.GetInvoice, "/invoices/list": h.ListInvoices,
		"/payments/create-attempt": h.CreatePaymentAttempt, "/payments/get": h.GetPayment, "/payments/apply-result": h.ApplyPaymentResult, "/payments/list": h.ListPayments,
		"/payments/refunds/record": h.RecordRefund, "/payments/refunds/list": h.ListRefunds,
	} {
		api.POST(path, handler)
	}
}

func decode[T any](h *Handler, c *gin.Context) (T, bool) {
	var request T
	if err := c.ShouldBindJSON(&request); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid JSON request", err))
		return request, false
	}
	return request, true
}
func result(h *Handler, c *gin.Context, value any, err error) {
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, value)
}

func (h *Handler) CreatePlan(c *gin.Context) {
	r, ok := decode[createPlanRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.CreatePlan(c.Request.Context(), billing.Plan{Code: r.Code, Name: r.Name, Description: r.Description, Currency: r.Currency, BillingInterval: r.BillingInterval, BaseAmountMinor: r.BaseAmountMinor, TrialDays: r.TrialDays, EntitlementsJSON: string(rawObject(r.EntitlementsJSON))})
	result(h, c, planBody(v), e)
}
func (h *Handler) UpdatePlan(c *gin.Context) {
	r, ok := decode[updatePlanRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.UpdatePlan(c.Request.Context(), billing.Plan{ID: r.ID, Name: r.Name, Description: r.Description, BaseAmountMinor: r.BaseAmountMinor, TrialDays: r.TrialDays, Status: r.Status, EntitlementsJSON: string(rawObject(r.EntitlementsJSON))}, r.Version)
	result(h, c, planBody(v), e)
}
func (h *Handler) GetPlan(c *gin.Context) {
	r, ok := decode[getPlanRequest](h, c)
	if !ok {
		return
	}
	v, p, e := h.billing.GetPlan(c.Request.Context(), r.ID, r.Code)
	result(h, c, PlanDetailBody{Plan: planBody(v), UsagePrices: mapBodies(p, usagePriceBody)}, e)
}
func (h *Handler) ListPlans(c *gin.Context) {
	r, ok := decode[listPlansRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.ListPlans(c.Request.Context(), r.Status, r.Keyword, r.Page, r.PageSize)
	result(h, c, planPage(v), e)
}
func (h *Handler) UpsertUsagePrice(c *gin.Context) {
	r, ok := decode[upsertUsagePriceRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.UpsertUsagePrice(c.Request.Context(), billing.UsagePrice{ID: r.ID, PlanID: r.PlanID, MeterCode: r.MeterCode, IncludedQuantity: r.IncludedQuantity, UnitQuantity: r.UnitQuantity, UnitAmountMinor: r.UnitAmountMinor, PricingModel: r.PricingModel, TiersJSON: string(rawArray(r.TiersJSON))}, r.ExpectedVersion)
	result(h, c, usagePriceBody(v), e)
}
func (h *Handler) DeleteUsagePrice(c *gin.Context) {
	r, ok := decode[idVersionRequest](h, c)
	if !ok {
		return
	}
	e := h.billing.DeleteUsagePrice(c.Request.Context(), r.ID, r.Version)
	result(h, c, nil, e)
}
func (h *Handler) CreateSubscription(c *gin.Context) {
	r, ok := decode[createSubscriptionRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.CreateSubscription(c.Request.Context(), r.TenantID, r.ApplicationID, r.PlanID, r.PlanVersion, r.StartsAt, r.ExternalReference)
	result(h, c, subscriptionBody(v), e)
}
func (h *Handler) ChangeSubscription(c *gin.Context) {
	r, ok := decode[changeSubscriptionRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.ChangeSubscription(c.Request.Context(), r.TenantID, r.ApplicationID, r.ID, r.PlanID, r.EffectiveMode, r.Version)
	result(h, c, subscriptionBody(v), e)
}
func (h *Handler) CancelSubscription(c *gin.Context) {
	r, ok := decode[cancelSubscriptionRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.CancelSubscription(c.Request.Context(), r.TenantID, r.ApplicationID, r.ID, r.AtPeriodEnd, r.Version)
	result(h, c, subscriptionBody(v), e)
}
func (h *Handler) GetSubscription(c *gin.Context) {
	r, ok := decode[getSubscriptionRequest](h, c)
	if !ok {
		return
	}
	v, p, e := h.billing.GetSubscription(c.Request.Context(), r.TenantID, r.ApplicationID, r.ID)
	result(h, c, SubscriptionDetailBody{Subscription: subscriptionBody(v), Plan: planBody(p)}, e)
}
func (h *Handler) ListSubscriptions(c *gin.Context) {
	r, ok := decode[listSubscriptionsRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.ListSubscriptions(c.Request.Context(), r.TenantID, r.ApplicationID, r.Status, r.Page, r.PageSize)
	result(h, c, subscriptionPage(v), e)
}
func (h *Handler) PreviewInvoice(c *gin.Context) {
	r, ok := decode[invoicePeriodRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.PreviewInvoice(c.Request.Context(), r.TenantID, r.ApplicationID, r.SubscriptionID, r.PeriodStart, r.PeriodEnd)
	result(h, c, InvoiceDetailBody{Invoice: invoiceBody(v.Invoice), Lines: mapBodies(v.Lines, invoiceLineBody)}, e)
}
func (h *Handler) GenerateInvoice(c *gin.Context) {
	r, ok := decode[invoicePeriodRequest](h, c)
	if !ok {
		return
	}
	v, d, e := h.billing.GenerateInvoice(c.Request.Context(), r.TenantID, r.ApplicationID, r.SubscriptionID, r.PeriodStart, r.PeriodEnd, r.IdempotencyKey)
	result(h, c, GenerateInvoiceBody{Invoice: invoiceBody(v.Invoice), Lines: mapBodies(v.Lines, invoiceLineBody), Duplicate: d}, e)
}
func (h *Handler) FinalizeInvoice(c *gin.Context) {
	r, ok := decode[finalizeInvoiceRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.FinalizeInvoice(c.Request.Context(), r.TenantID, r.ApplicationID, r.ID, r.DueAt, r.Version)
	result(h, c, invoiceBody(v), e)
}
func (h *Handler) VoidInvoice(c *gin.Context) {
	r, ok := decode[voidInvoiceRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.VoidInvoice(c.Request.Context(), r.TenantID, r.ApplicationID, r.ID, r.Reason, r.Version)
	result(h, c, invoiceBody(v), e)
}
func (h *Handler) GetInvoice(c *gin.Context) {
	r, ok := decode[getInvoiceRequest](h, c)
	if !ok {
		return
	}
	v, l, e := h.billing.GetInvoice(c.Request.Context(), r.TenantID, r.ApplicationID, r.ID)
	result(h, c, InvoiceDetailBody{Invoice: invoiceBody(v), Lines: mapBodies(l, invoiceLineBody)}, e)
}

func rawObject(value json.RawMessage) json.RawMessage { return rawJSON(value, `{}`) }
func rawArray(value json.RawMessage) json.RawMessage  { return rawJSON(value, `[]`) }

func rawJSON(value json.RawMessage, fallback string) json.RawMessage {
	if len(value) > 0 && json.Valid(value) {
		if value[0] == '"' {
			var legacy string
			if json.Unmarshal(value, &legacy) == nil && json.Valid([]byte(legacy)) {
				return json.RawMessage(legacy)
			}
		}
		return value
	}
	return json.RawMessage(fallback)
}

func (h *Handler) ListInvoices(c *gin.Context) {
	r, ok := decode[listInvoicesRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.ListInvoices(c.Request.Context(), r.TenantID, r.ApplicationID, r.Status, r.CreatedFrom, r.CreatedTo, r.Page, r.PageSize)
	result(h, c, invoicePage(v), e)
}
func (h *Handler) CreatePaymentAttempt(c *gin.Context) {
	r, ok := decode[createPaymentRequest](h, c)
	if !ok {
		return
	}
	v, d, e := h.billing.CreatePaymentAttempt(c.Request.Context(), r.TenantID, r.ApplicationID, r.InvoiceID, r.Provider, r.PaymentMethodReference, r.IdempotencyKey)
	result(h, c, CreatePaymentAttemptBody{PaymentAttempt: paymentAttemptBody(v), Duplicate: d}, e)
}
func (h *Handler) ListPayments(c *gin.Context) {
	r, ok := decode[listPaymentsRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.ListPayments(c.Request.Context(), r.TenantID, r.ApplicationID, r.Status, r.Page, r.PageSize)
	result(h, c, paymentAttemptPage(v), e)
}
func (h *Handler) GetPayment(c *gin.Context) {
	r, ok := decode[getPaymentRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.GetPayment(c.Request.Context(), r.TenantID, r.ApplicationID, r.ID)
	result(h, c, paymentAttemptBody(v), e)
}
func (h *Handler) ApplyPaymentResult(c *gin.Context) {
	r, ok := decode[applyPaymentRequest](h, c)
	if !ok {
		return
	}
	p, i, d, e := h.billing.ApplyPaymentResult(c.Request.Context(), r.PaymentAttemptID, r.ProviderPaymentID, r.ProviderEventID, r.Status, r.FailureCode, r.FailureMessage, r.ProcessedAt)
	result(h, c, ApplyPaymentResultBody{PaymentAttempt: paymentAttemptBody(p), Invoice: invoiceBody(i), Duplicate: d}, e)
}
func (h *Handler) RecordRefund(c *gin.Context) {
	r, ok := decode[recordRefundRequest](h, c)
	if !ok {
		return
	}
	v, i, d, e := h.billing.RecordRefund(c.Request.Context(), r.TenantID, r.ApplicationID, r.PaymentAttemptID, r.ProviderRefundID, r.IdempotencyKey, r.AmountMinor, r.Reason, r.Status)
	result(h, c, RecordRefundBody{Refund: refundBody(v), Invoice: invoiceBody(i), Duplicate: d}, e)
}
func (h *Handler) ListRefunds(c *gin.Context) {
	r, ok := decode[listPaymentsRequest](h, c)
	if !ok {
		return
	}
	v, e := h.billing.ListRefunds(c.Request.Context(), r.TenantID, r.ApplicationID, r.Status, r.Page, r.PageSize)
	result(h, c, refundPage(v), e)
}
