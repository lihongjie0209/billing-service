package billing

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound     = errors.New("billing resource not found")
	ErrStaleVersion = errors.New("stale billing resource version")
	ErrConflict     = errors.New("billing resource conflict")
)

type Repository interface {
	CreatePlan(context.Context, sqlx.ExtContext, Plan) error
	UpdatePlan(context.Context, sqlx.ExtContext, Plan, int64) error
	GetPlan(context.Context, string, string) (Plan, error)
	LockActivePlan(context.Context, sqlx.ExtContext, string, int64) (Plan, error)
	ListPlans(context.Context, string, string, int, int) ([]Plan, int64, error)
	UpsertUsagePrice(context.Context, sqlx.ExtContext, UsagePrice, int64) error
	DeleteUsagePrice(context.Context, sqlx.ExtContext, string, int64) error
	ListUsagePrices(context.Context, string) ([]UsagePrice, error)
	CreateSubscription(context.Context, sqlx.ExtContext, Subscription) error
	UpdateSubscription(context.Context, sqlx.ExtContext, Subscription, int64) error
	GetSubscription(context.Context, string, string, string) (Subscription, error)
	ListSubscriptions(context.Context, string, string, string, int, int) ([]Subscription, int64, error)
	ClaimSubscription(context.Context, sqlx.ExtContext, string, string, string, Audit) error
	ReleaseSubscriptionClaim(context.Context, sqlx.ExtContext, string, string, string) error
	ListDueSubscriptions(context.Context, time.Time, int) ([]Subscription, error)
	ClaimInvoice(context.Context, sqlx.ExtContext, string, string, string, string, string, Audit) (string, bool, error)
	CreateInvoice(context.Context, sqlx.ExtContext, Invoice, []InvoiceLine) error
	UpdateInvoice(context.Context, sqlx.ExtContext, Invoice, int64) error
	GetInvoice(context.Context, string, string, string) (Invoice, []InvoiceLine, error)
	LockPayableInvoice(context.Context, sqlx.ExtContext, string, string, string, int64) (Invoice, error)
	ListInvoices(context.Context, string, string, string, time.Time, time.Time, int, int) ([]Invoice, int64, error)
	ListPayableInvoices(context.Context, string, string, string, int, int) ([]Invoice, int64, error)
	ClaimPayment(context.Context, sqlx.ExtContext, PaymentAttempt) (string, bool, error)
	GetPaymentByKey(context.Context, string) (PaymentAttempt, error)
	GetPayment(context.Context, string, string, string) (PaymentAttempt, error)
	ListPayments(context.Context, string, string, string, int, int) ([]PaymentAttempt, int64, error)
	UpdatePayment(context.Context, sqlx.ExtContext, PaymentAttempt, int64) error
	ClaimProviderEvent(context.Context, sqlx.ExtContext, string, string, string, Audit) (bool, error)
	ClaimRefund(context.Context, sqlx.ExtContext, Refund) (string, bool, error)
	GetRefund(context.Context, string) (Refund, error)
	ListRefunds(context.Context, string, string, string, int, int) ([]Refund, int64, error)
	AddOutbox(context.Context, sqlx.ExtContext, OutboxEvent) error
}

type OutboxEvent struct {
	ID          string
	Subject     string
	Envelope    []byte
	AvailableAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
	UpdatedBy   string
}

type SQLRepository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &SQLRepository{db: db} }

const planColumns = "id,code,name,description,currency,billing_interval,base_amount_minor,trial_days,status,entitlements_json,version,created_at,updated_at,created_by,updated_by"
const usagePriceColumns = "id,plan_id,meter_code,included_quantity,unit_quantity,unit_amount_minor,pricing_model,tiers_json,version,created_at,updated_at,created_by,updated_by"
const subscriptionColumns = "id,tenant_id,application_id,plan_id,status,current_period_start,current_period_end,cancel_at_period_end,canceled_at,external_reference,pending_plan_id,pending_change_at,version,created_at,updated_at,created_by,updated_by"
const invoiceColumns = "id,number,tenant_id,application_id,subscription_id,currency,status,period_start,period_end,subtotal_minor,discount_minor,tax_minor,total_minor,paid_minor,refunded_minor,due_at,finalized_at,paid_at,version,created_at,updated_at,created_by,updated_by"
const invoiceLineColumns = "id,invoice_id,type,description,meter_code,quantity,unit_quantity,unit_amount_minor,amount_minor,metadata_json,version,created_at,updated_at,created_by,updated_by"
const paymentColumns = "id,invoice_id,tenant_id,application_id,provider,provider_payment_id,idempotency_key,request_hash,currency,amount_minor,status,failure_code,failure_message,processed_at,version,created_at,updated_at,created_by,updated_by"
const refundColumns = "id,payment_attempt_id,invoice_id,tenant_id,application_id,provider_refund_id,idempotency_key,request_hash,amount_minor,reason,status,version,created_at,updated_at,created_by,updated_by"
const refundInsertValues = "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"

func (r *SQLRepository) CreatePlan(ctx context.Context, e sqlx.ExtContext, v Plan) error {
	_, err := e.ExecContext(ctx, r.db.Rebind("INSERT INTO plans ("+planColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"), v.ID, v.Code, v.Name, v.Description, v.Currency, v.BillingInterval, v.BaseAmountMinor, v.TrialDays, v.Status, v.EntitlementsJSON, v.Version, v.CreatedAt, v.UpdatedAt, v.CreatedBy, v.UpdatedBy)
	return err
}
func (r *SQLRepository) UpdatePlan(ctx context.Context, e sqlx.ExtContext, v Plan, expected int64) error {
	result, err := e.ExecContext(ctx, r.db.Rebind("UPDATE plans SET name=?,description=?,base_amount_minor=?,trial_days=?,status=?,entitlements_json=?,version=version+1,updated_at=?,updated_by=? WHERE id=? AND version=?"), v.Name, v.Description, v.BaseAmountMinor, v.TrialDays, v.Status, v.EntitlementsJSON, v.UpdatedAt, v.UpdatedBy, v.ID, expected)
	return optimistic(result, err)
}
func (r *SQLRepository) GetPlan(ctx context.Context, id, code string) (Plan, error) {
	query, arg := "SELECT "+planColumns+" FROM plans WHERE id=?", id
	if strings.TrimSpace(id) == "" {
		query, arg = "SELECT "+planColumns+" FROM plans WHERE code=?", code
	}
	var v Plan
	err := r.db.GetContext(ctx, &v, r.db.Rebind(query), arg)
	return v, notFound(err)
}

func (r *SQLRepository) LockActivePlan(ctx context.Context, e sqlx.ExtContext, id string, expectedVersion int64) (Plan, error) {
	var value Plan
	err := sqlx.GetContext(ctx, e, &value, r.db.Rebind("SELECT "+planColumns+" FROM plans WHERE id=? FOR UPDATE"), id)
	if err != nil {
		return Plan{}, notFound(err)
	}
	if value.Version != expectedVersion || value.Status != "active" {
		return Plan{}, ErrStaleVersion
	}
	return value, nil
}
func (r *SQLRepository) ListPlans(ctx context.Context, status, keyword string, limit, offset int) ([]Plan, int64, error) {
	where, args := "1=1", []any{}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	if keyword != "" {
		where += " AND (LOWER(code) LIKE ? OR LOWER(name) LIKE ?)"
		like := "%" + strings.ToLower(keyword) + "%"
		args = append(args, like, like)
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, r.db.Rebind("SELECT COUNT(*) FROM plans WHERE "+where), args...); err != nil {
		return nil, 0, err
	}
	items := []Plan{}
	pageArgs := append(append([]any{}, args...), limit, offset)
	err := r.db.SelectContext(ctx, &items, r.db.Rebind("SELECT "+planColumns+" FROM plans WHERE "+where+" ORDER BY code LIMIT ? OFFSET ?"), pageArgs...)
	return items, total, err
}
func (r *SQLRepository) UpsertUsagePrice(ctx context.Context, e sqlx.ExtContext, v UsagePrice, expected int64) error {
	if expected == 0 {
		_, err := e.ExecContext(ctx, r.db.Rebind("INSERT INTO usage_prices ("+usagePriceColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)"), v.ID, v.PlanID, v.MeterCode, v.IncludedQuantity, v.UnitQuantity, v.UnitAmountMinor, v.PricingModel, v.TiersJSON, v.Version, v.CreatedAt, v.UpdatedAt, v.CreatedBy, v.UpdatedBy)
		return err
	}
	result, err := e.ExecContext(ctx, r.db.Rebind("UPDATE usage_prices SET included_quantity=?,unit_quantity=?,unit_amount_minor=?,pricing_model=?,tiers_json=?,version=version+1,updated_at=?,updated_by=? WHERE id=? AND version=?"), v.IncludedQuantity, v.UnitQuantity, v.UnitAmountMinor, v.PricingModel, v.TiersJSON, v.UpdatedAt, v.UpdatedBy, v.ID, expected)
	return optimistic(result, err)
}
func (r *SQLRepository) DeleteUsagePrice(ctx context.Context, e sqlx.ExtContext, id string, expected int64) error {
	result, err := e.ExecContext(ctx, r.db.Rebind("DELETE FROM usage_prices WHERE id=? AND version=?"), id, expected)
	return optimistic(result, err)
}
func (r *SQLRepository) ListUsagePrices(ctx context.Context, planID string) ([]UsagePrice, error) {
	items := []UsagePrice{}
	err := r.db.SelectContext(ctx, &items, r.db.Rebind("SELECT "+usagePriceColumns+" FROM usage_prices WHERE plan_id=? ORDER BY meter_code"), planID)
	return items, err
}
func (r *SQLRepository) CreateSubscription(ctx context.Context, e sqlx.ExtContext, v Subscription) error {
	_, err := e.ExecContext(ctx, r.db.Rebind("INSERT INTO subscriptions ("+subscriptionColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"), v.ID, v.TenantID, v.ApplicationID, v.PlanID, v.Status, v.CurrentPeriodStart, v.CurrentPeriodEnd, v.CancelAtPeriodEnd, v.CanceledAt, v.ExternalReference, v.PendingPlanID, v.PendingChangeAt, v.Version, v.CreatedAt, v.UpdatedAt, v.CreatedBy, v.UpdatedBy)
	return err
}
func (r *SQLRepository) UpdateSubscription(ctx context.Context, e sqlx.ExtContext, v Subscription, expected int64) error {
	result, err := e.ExecContext(ctx, r.db.Rebind("UPDATE subscriptions SET plan_id=?,status=?,current_period_start=?,current_period_end=?,cancel_at_period_end=?,canceled_at=?,pending_plan_id=?,pending_change_at=?,version=version+1,updated_at=?,updated_by=? WHERE id=? AND version=?"), v.PlanID, v.Status, v.CurrentPeriodStart, v.CurrentPeriodEnd, v.CancelAtPeriodEnd, v.CanceledAt, v.PendingPlanID, v.PendingChangeAt, v.UpdatedAt, v.UpdatedBy, v.ID, expected)
	return optimistic(result, err)
}
func (r *SQLRepository) GetSubscription(ctx context.Context, tenantID, applicationID, id string) (Subscription, error) {
	var v Subscription
	err := r.db.GetContext(ctx, &v, r.db.Rebind("SELECT "+subscriptionColumns+" FROM subscriptions WHERE tenant_id=? AND application_id=? AND id=?"), tenantID, applicationID, id)
	return v, notFound(err)
}
func (r *SQLRepository) ListSubscriptions(ctx context.Context, tenantID, applicationID, status string, limit, offset int) ([]Subscription, int64, error) {
	where, args := "tenant_id=? AND application_id=?", []any{tenantID, applicationID}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, r.db.Rebind("SELECT COUNT(*) FROM subscriptions WHERE "+where), args...); err != nil {
		return nil, 0, err
	}
	items := []Subscription{}
	pageArgs := append(append([]any{}, args...), limit, offset)
	err := r.db.SelectContext(ctx, &items, r.db.Rebind("SELECT "+subscriptionColumns+" FROM subscriptions WHERE "+where+" ORDER BY created_at DESC,id LIMIT ? OFFSET ?"), pageArgs...)
	return items, total, err
}
func (r *SQLRepository) ClaimSubscription(ctx context.Context, e sqlx.ExtContext, tenantID, applicationID, subscriptionID string, a Audit) error {
	_, err := e.ExecContext(ctx, r.db.Rebind("INSERT INTO subscription_claims (tenant_id,application_id,subscription_id,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,1,?,?,?,?)"), tenantID, applicationID, subscriptionID, a.CreatedAt, a.UpdatedAt, a.CreatedBy, a.UpdatedBy)
	if err != nil {
		return ErrConflict
	}
	return nil
}
func (r *SQLRepository) ReleaseSubscriptionClaim(ctx context.Context, e sqlx.ExtContext, tenantID, applicationID, subscriptionID string) error {
	_, err := e.ExecContext(ctx, r.db.Rebind("DELETE FROM subscription_claims WHERE tenant_id=? AND application_id=? AND subscription_id=?"), tenantID, applicationID, subscriptionID)
	return err
}
func (r *SQLRepository) ListDueSubscriptions(ctx context.Context, now time.Time, limit int) ([]Subscription, error) {
	items := []Subscription{}
	err := r.db.SelectContext(ctx, &items, r.db.Rebind("SELECT "+subscriptionColumns+" FROM subscriptions WHERE (pending_change_at IS NOT NULL AND pending_change_at<=?) OR (cancel_at_period_end=? AND current_period_end<=?) ORDER BY current_period_end,id LIMIT ?"), now, true, now, limit)
	return items, err
}
func (r *SQLRepository) ClaimInvoice(ctx context.Context, e sqlx.ExtContext, key, invoiceID, tenantID, applicationID, requestHash string, a Audit) (string, bool, error) {
	query := "INSERT INTO invoice_generation_keys (idempotency_key,invoice_id,tenant_id,application_id,request_hash,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,?,?,1,?,?,?,?) ON CONFLICT (idempotency_key) DO NOTHING"
	if r.db.DriverName() == "mysql" {
		query = "INSERT IGNORE INTO invoice_generation_keys (idempotency_key,invoice_id,tenant_id,application_id,request_hash,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,?,?,1,?,?,?,?)"
	}
	result, err := e.ExecContext(ctx, r.db.Rebind(query), key, invoiceID, tenantID, applicationID, requestHash, a.CreatedAt, a.UpdatedAt, a.CreatedBy, a.UpdatedBy)
	if err != nil {
		return "", false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return "", false, err
	}
	if rows == 1 {
		return invoiceID, true, nil
	}
	var existingID, existingHash string
	row := e.QueryRowxContext(ctx, r.db.Rebind("SELECT invoice_id,request_hash FROM invoice_generation_keys WHERE idempotency_key=?"), key)
	if err := row.Scan(&existingID, &existingHash); err != nil {
		return "", false, err
	}
	if existingHash != requestHash {
		return "", false, ErrConflict
	}
	return existingID, false, nil
}
func (r *SQLRepository) CreateInvoice(ctx context.Context, e sqlx.ExtContext, v Invoice, lines []InvoiceLine) error {
	_, err := e.ExecContext(ctx, r.db.Rebind("INSERT INTO invoices ("+invoiceColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"), v.ID, v.Number, v.TenantID, v.ApplicationID, v.SubscriptionID, v.Currency, v.Status, v.PeriodStart, v.PeriodEnd, v.SubtotalMinor, v.DiscountMinor, v.TaxMinor, v.TotalMinor, v.PaidMinor, v.RefundedMinor, v.DueAt, v.FinalizedAt, v.PaidAt, v.Version, v.CreatedAt, v.UpdatedAt, v.CreatedBy, v.UpdatedBy)
	if err != nil {
		return err
	}
	for _, line := range lines {
		_, err = e.ExecContext(ctx, r.db.Rebind("INSERT INTO invoice_lines ("+invoiceLineColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"), line.ID, line.InvoiceID, line.Type, line.Description, line.MeterCode, line.Quantity, line.UnitQuantity, line.UnitAmountMinor, line.AmountMinor, line.MetadataJSON, line.Version, line.CreatedAt, line.UpdatedAt, line.CreatedBy, line.UpdatedBy)
		if err != nil {
			return err
		}
	}
	return nil
}
func (r *SQLRepository) UpdateInvoice(ctx context.Context, e sqlx.ExtContext, v Invoice, expected int64) error {
	result, err := e.ExecContext(ctx, r.db.Rebind("UPDATE invoices SET status=?,paid_minor=?,refunded_minor=?,due_at=?,finalized_at=?,paid_at=?,version=version+1,updated_at=?,updated_by=? WHERE id=? AND version=?"), v.Status, v.PaidMinor, v.RefundedMinor, v.DueAt, v.FinalizedAt, v.PaidAt, v.UpdatedAt, v.UpdatedBy, v.ID, expected)
	return optimistic(result, err)
}
func (r *SQLRepository) GetInvoice(ctx context.Context, tenantID, applicationID, id string) (Invoice, []InvoiceLine, error) {
	var v Invoice
	err := r.db.GetContext(ctx, &v, r.db.Rebind("SELECT "+invoiceColumns+" FROM invoices WHERE tenant_id=? AND application_id=? AND id=?"), tenantID, applicationID, id)
	if err != nil {
		return Invoice{}, nil, notFound(err)
	}
	lines := []InvoiceLine{}
	err = r.db.SelectContext(ctx, &lines, r.db.Rebind("SELECT "+invoiceLineColumns+" FROM invoice_lines WHERE invoice_id=? ORDER BY id"), id)
	return v, lines, err
}

func (r *SQLRepository) LockPayableInvoice(ctx context.Context, e sqlx.ExtContext, tenantID, applicationID, id string, expectedVersion int64) (Invoice, error) {
	var value Invoice
	err := sqlx.GetContext(ctx, e, &value, r.db.Rebind("SELECT "+invoiceColumns+" FROM invoices WHERE tenant_id=? AND application_id=? AND id=? FOR UPDATE"), tenantID, applicationID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Invoice{}, ErrNotFound
	}
	if err != nil {
		return Invoice{}, err
	}
	if value.Version != expectedVersion || value.Status != "open" || value.TotalMinor-value.PaidMinor <= 0 {
		return Invoice{}, ErrStaleVersion
	}
	return value, nil
}
func (r *SQLRepository) ListInvoices(ctx context.Context, tenantID, applicationID, status string, from, to time.Time, limit, offset int) ([]Invoice, int64, error) {
	where, args := "tenant_id=? AND application_id=?", []any{tenantID, applicationID}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	if !from.IsZero() {
		where += " AND created_at>=?"
		args = append(args, from)
	}
	if !to.IsZero() {
		where += " AND created_at<?"
		args = append(args, to)
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, r.db.Rebind("SELECT COUNT(*) FROM invoices WHERE "+where), args...); err != nil {
		return nil, 0, err
	}
	items := []Invoice{}
	pageArgs := append(append([]any{}, args...), limit, offset)
	err := r.db.SelectContext(ctx, &items, r.db.Rebind("SELECT "+invoiceColumns+" FROM invoices WHERE "+where+" ORDER BY created_at DESC,id LIMIT ? OFFSET ?"), pageArgs...)
	return items, total, err
}

func (r *SQLRepository) ListPayableInvoices(ctx context.Context, tenantID, applicationID, keyword string, limit, offset int) ([]Invoice, int64, error) {
	where, args := "tenant_id=? AND application_id=? AND status='open'", []any{tenantID, applicationID}
	if keyword != "" {
		where += " AND (LOWER(number) LIKE ? OR LOWER(id) LIKE ?)"
		like := "%" + strings.ToLower(keyword) + "%"
		args = append(args, like, like)
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, r.db.Rebind("SELECT COUNT(*) FROM invoices WHERE "+where), args...); err != nil {
		return nil, 0, err
	}
	items := []Invoice{}
	pageArgs := append(append([]any{}, args...), limit, offset)
	err := r.db.SelectContext(ctx, &items, r.db.Rebind("SELECT "+invoiceColumns+" FROM invoices WHERE "+where+" ORDER BY created_at DESC LIMIT ? OFFSET ?"), pageArgs...)
	return items, total, err
}
func (r *SQLRepository) ClaimPayment(ctx context.Context, e sqlx.ExtContext, v PaymentAttempt) (string, bool, error) {
	query := "INSERT INTO payment_attempts (" + paymentColumns + ") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT (idempotency_key) DO NOTHING"
	if r.db.DriverName() == "mysql" {
		query = "INSERT IGNORE INTO payment_attempts (" + paymentColumns + ") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
	}
	result, err := e.ExecContext(ctx, r.db.Rebind(query), v.ID, v.InvoiceID, v.TenantID, v.ApplicationID, v.Provider, v.ProviderPaymentID, v.IdempotencyKey, v.RequestHash, v.Currency, v.AmountMinor, v.Status, v.FailureCode, v.FailureMessage, v.ProcessedAt, v.Version, v.CreatedAt, v.UpdatedAt, v.CreatedBy, v.UpdatedBy)
	if err != nil {
		return "", false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return "", false, err
	}
	if rows == 1 {
		return v.ID, true, nil
	}
	var existingID, existingHash string
	if err := e.QueryRowxContext(ctx, r.db.Rebind("SELECT id,request_hash FROM payment_attempts WHERE idempotency_key=?"), v.IdempotencyKey).Scan(&existingID, &existingHash); err != nil {
		return "", false, err
	}
	if existingHash != v.RequestHash {
		return "", false, ErrConflict
	}
	return existingID, false, nil
}
func (r *SQLRepository) GetPayment(ctx context.Context, tenantID, applicationID, id string) (PaymentAttempt, error) {
	query, args := "SELECT "+paymentColumns+" FROM payment_attempts WHERE id=?", []any{id}
	if tenantID != "" {
		query, args = "SELECT "+paymentColumns+" FROM payment_attempts WHERE tenant_id=? AND application_id=? AND id=?", []any{tenantID, applicationID, id}
	}
	var v PaymentAttempt
	err := r.db.GetContext(ctx, &v, r.db.Rebind(query), args...)
	return v, notFound(err)
}
func (r *SQLRepository) ListPayments(ctx context.Context, tenantID, applicationID, status string, limit, offset int) ([]PaymentAttempt, int64, error) {
	where, args := "tenant_id=? AND application_id=?", []any{tenantID, applicationID}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, r.db.Rebind("SELECT COUNT(*) FROM payment_attempts WHERE "+where), args...); err != nil {
		return nil, 0, err
	}
	items := []PaymentAttempt{}
	pageArgs := append(append([]any{}, args...), limit, offset)
	err := r.db.SelectContext(ctx, &items, r.db.Rebind("SELECT "+paymentColumns+" FROM payment_attempts WHERE "+where+" ORDER BY created_at DESC,id LIMIT ? OFFSET ?"), pageArgs...)
	return items, total, err
}
func (r *SQLRepository) GetPaymentByKey(ctx context.Context, key string) (PaymentAttempt, error) {
	var v PaymentAttempt
	err := r.db.GetContext(ctx, &v, r.db.Rebind("SELECT "+paymentColumns+" FROM payment_attempts WHERE idempotency_key=?"), key)
	return v, notFound(err)
}
func (r *SQLRepository) UpdatePayment(ctx context.Context, e sqlx.ExtContext, v PaymentAttempt, expected int64) error {
	result, err := e.ExecContext(ctx, r.db.Rebind("UPDATE payment_attempts SET provider_payment_id=?,status=?,failure_code=?,failure_message=?,processed_at=?,version=version+1,updated_at=?,updated_by=? WHERE id=? AND version=?"), v.ProviderPaymentID, v.Status, v.FailureCode, v.FailureMessage, v.ProcessedAt, v.UpdatedAt, v.UpdatedBy, v.ID, expected)
	return optimistic(result, err)
}
func (r *SQLRepository) ClaimProviderEvent(ctx context.Context, e sqlx.ExtContext, provider, eventID, paymentID string, a Audit) (bool, error) {
	query := "INSERT INTO payment_provider_events (provider,event_id,payment_attempt_id,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,1,?,?,?,?) ON CONFLICT (provider,event_id) DO NOTHING"
	if r.db.DriverName() == "mysql" {
		query = "INSERT IGNORE INTO payment_provider_events (provider,event_id,payment_attempt_id,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,1,?,?,?,?)"
	}
	result, err := e.ExecContext(ctx, r.db.Rebind(query), provider, eventID, paymentID, a.CreatedAt, a.UpdatedAt, a.CreatedBy, a.UpdatedBy)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}
func (r *SQLRepository) ClaimRefund(ctx context.Context, e sqlx.ExtContext, v Refund) (string, bool, error) {
	query := "INSERT INTO refunds (" + refundColumns + ") VALUES " + refundInsertValues + " ON CONFLICT (idempotency_key) DO NOTHING"
	if r.db.DriverName() == "mysql" {
		query = "INSERT IGNORE INTO refunds (" + refundColumns + ") VALUES " + refundInsertValues
	}
	result, err := e.ExecContext(ctx, r.db.Rebind(query), v.ID, v.PaymentAttemptID, v.InvoiceID, v.TenantID, v.ApplicationID, v.ProviderRefundID, v.IdempotencyKey, v.RequestHash, v.AmountMinor, v.Reason, v.Status, v.Version, v.CreatedAt, v.UpdatedAt, v.CreatedBy, v.UpdatedBy)
	if err != nil {
		return "", false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return "", false, err
	}
	if rows == 1 {
		return v.ID, true, nil
	}
	var existingID, existingHash string
	if err := e.QueryRowxContext(ctx, r.db.Rebind("SELECT id,request_hash FROM refunds WHERE idempotency_key=?"), v.IdempotencyKey).Scan(&existingID, &existingHash); err != nil {
		return "", false, err
	}
	if existingHash != v.RequestHash {
		return "", false, ErrConflict
	}
	return existingID, false, nil
}
func (r *SQLRepository) GetRefund(ctx context.Context, id string) (Refund, error) {
	var v Refund
	err := r.db.GetContext(ctx, &v, r.db.Rebind("SELECT "+refundColumns+" FROM refunds WHERE id=?"), id)
	return v, notFound(err)
}
func (r *SQLRepository) ListRefunds(ctx context.Context, tenantID, applicationID, status string, limit, offset int) ([]Refund, int64, error) {
	where, args := "tenant_id=? AND application_id=?", []any{tenantID, applicationID}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, r.db.Rebind("SELECT COUNT(*) FROM refunds WHERE "+where), args...); err != nil {
		return nil, 0, err
	}
	items := []Refund{}
	pageArgs := append(append([]any{}, args...), limit, offset)
	err := r.db.SelectContext(ctx, &items, r.db.Rebind("SELECT "+refundColumns+" FROM refunds WHERE "+where+" ORDER BY created_at DESC,id LIMIT ? OFFSET ?"), pageArgs...)
	return items, total, err
}
func (r *SQLRepository) AddOutbox(ctx context.Context, e sqlx.ExtContext, v OutboxEvent) error {
	_, err := e.ExecContext(ctx, r.db.Rebind("INSERT INTO billing_outbox_events (id,subject,envelope,attempts,available_at,published_at,last_error,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,0,?,NULL,'',1,?,?,?,?)"), v.ID, v.Subject, v.Envelope, v.AvailableAt, v.CreatedAt, v.UpdatedAt, v.CreatedBy, v.UpdatedBy)
	return err
}
func optimistic(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return ErrStaleVersion
	}
	return err
}
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
