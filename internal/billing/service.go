package billing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lihongjie0209/billing-service/internal/apperror"
	"github.com/lihongjie0209/billing-service/internal/database"
	"github.com/lihongjie0209/microservice-platform-go/appaccess"
	platformevents "github.com/lihongjie0209/microservice-platform-go/eventbus"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	billingv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/billing/v1"
	"google.golang.org/protobuf/proto"
)

var planCodePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{1,127}$`)
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

type UsageReader interface {
	Total(context.Context, string, string, string, time.Time, time.Time) (int64, error)
}

type Service struct {
	repository   Repository
	transactor   transactionRunner
	usage        UsageReader
	gateway      PaymentGateway
	applications appaccess.Verifier
	now          func() time.Time
}

type transactionRunner interface {
	Within(context.Context, *sql.TxOptions, func(*sqlx.Tx) error) error
}

func NewService(repository Repository, transactor *database.Transactor, usage UsageReader, applications appaccess.Verifier) (*Service, error) {
	if repository == nil || applications == nil {
		return nil, errors.New("billing repository and application verifier are required")
	}
	return &Service{repository: repository, transactor: transactor, usage: usage, applications: applications, now: time.Now}, nil
}

func NewRuntimeService(repository Repository, transactor *database.Transactor, usage UsageReader, gateway PaymentGateway, applications appaccess.Verifier) (*Service, error) {
	service, err := NewService(repository, transactor, usage, applications)
	if err != nil {
		return nil, err
	}
	service.gateway = gateway
	return service, nil
}

func (s *Service) CreatePlan(ctx context.Context, value Plan) (Plan, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return Plan{}, err
	}
	value, err = normalizeNewPlan(value)
	if err != nil {
		return Plan{}, err
	}
	now := s.now()
	value.ID = uuid.NewString()
	value.Status = "draft"
	value.Audit = newAudit(actorID, now)
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if err := s.repository.CreatePlan(ctx, tx, value); err != nil {
			return err
		}
		return s.addEvent(ctx, tx, "platform.billing.plan.changed.v1", "platform.billing.v1.PlanChanged", value.ID, "plan", "", "", actorID, now, &billingv1.PlanChangedEvent{Plan: ToProtoPlan(value), ChangeType: "created"})
	})
	return value, translate(err)
}

func normalizeNewPlan(value Plan) (Plan, error) {
	value.Code = strings.ToLower(strings.TrimSpace(value.Code))
	value.Name = strings.TrimSpace(value.Name)
	value.Currency = strings.ToUpper(strings.TrimSpace(value.Currency))
	value.BillingInterval = strings.ToLower(strings.TrimSpace(value.BillingInterval))
	if !planCodePattern.MatchString(value.Code) || value.Name == "" || !currencyPattern.MatchString(value.Currency) || !validInterval(value.BillingInterval) || value.BaseAmountMinor < 0 || value.TrialDays < 0 || !validJSON(value.EntitlementsJSON, "{}") {
		return Plan{}, apperror.Invalid("invalid plan definition", nil)
	}
	value.EntitlementsJSON = compactJSON(defaultJSON(value.EntitlementsJSON, "{}"))
	return value, nil
}

// NormalizeImportedPlan applies the same validation and canonicalization as
// CreatePlan without performing a write.
func NormalizeImportedPlan(value Plan) (Plan, error) { return normalizeNewPlan(value) }

// ImportPlan creates a draft plan and treats a replay of identical plan data
// as success. The unique plan code is the durable idempotency boundary.
func (s *Service) ImportPlan(ctx context.Context, value Plan) (Plan, bool, error) {
	normalized, err := normalizeNewPlan(value)
	if err != nil {
		return Plan{}, false, err
	}
	if current, getErr := s.repository.GetPlan(ctx, "", normalized.Code); getErr == nil {
		if sameImportedPlan(current, normalized) {
			return current, true, nil
		}
		return Plan{}, false, apperror.Conflict("plan code already exists with different data", nil)
	} else if !errors.Is(getErr, ErrNotFound) {
		return Plan{}, false, translate(getErr)
	}
	created, err := s.CreatePlan(ctx, normalized)
	if err == nil {
		return created, false, nil
	}
	current, getErr := s.repository.GetPlan(ctx, "", normalized.Code)
	if getErr == nil && sameImportedPlan(current, normalized) {
		return current, true, nil
	}
	return Plan{}, false, err
}

func sameImportedPlan(current, candidate Plan) bool {
	return current.Code == candidate.Code && current.Name == candidate.Name && current.Description == candidate.Description && current.Currency == candidate.Currency && current.BillingInterval == candidate.BillingInterval && current.BaseAmountMinor == candidate.BaseAmountMinor && current.TrialDays == candidate.TrialDays && compactJSON(current.EntitlementsJSON) == compactJSON(candidate.EntitlementsJSON)
}
func (s *Service) UpdatePlan(ctx context.Context, value Plan, expected int64) (Plan, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return Plan{}, err
	}
	if expected < 1 || strings.TrimSpace(value.Name) == "" || value.BaseAmountMinor < 0 || value.TrialDays < 0 || !map[string]bool{"draft": true, "active": true, "retired": true}[value.Status] || !validJSON(value.EntitlementsJSON, "{}") {
		return Plan{}, apperror.Invalid("invalid plan update", nil)
	}
	current, err := s.repository.GetPlan(ctx, strings.TrimSpace(value.ID), "")
	if err != nil {
		return Plan{}, translate(err)
	}
	current.Name = strings.TrimSpace(value.Name)
	current.Description = strings.TrimSpace(value.Description)
	current.BaseAmountMinor = value.BaseAmountMinor
	current.TrialDays = value.TrialDays
	current.Status = value.Status
	current.EntitlementsJSON = defaultJSON(value.EntitlementsJSON, "{}")
	current.Version = expected + 1
	current.UpdatedAt = s.now()
	current.UpdatedBy = actorID
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if err := s.repository.UpdatePlan(ctx, tx, current, expected); err != nil {
			return err
		}
		return s.addEvent(ctx, tx, "platform.billing.plan.changed.v1", "platform.billing.v1.PlanChanged", current.ID, "plan", "", "", actorID, current.UpdatedAt, &billingv1.PlanChangedEvent{Plan: ToProtoPlan(current), ChangeType: "updated"})
	})
	return current, translate(err)
}
func (s *Service) GetPlan(ctx context.Context, id, code string) (Plan, []UsagePrice, error) {
	v, err := s.repository.GetPlan(ctx, strings.TrimSpace(id), strings.ToLower(strings.TrimSpace(code)))
	if err != nil {
		return Plan{}, nil, translate(err)
	}
	prices, err := s.repository.ListUsagePrices(ctx, v.ID)
	return v, prices, translate(err)
}
func (s *Service) ListPlans(ctx context.Context, status, keyword string, page, size int) (Page[Plan], error) {
	page, size, err := pagination(page, size)
	if err != nil {
		return Page[Plan]{}, err
	}
	items, total, err := s.repository.ListPlans(ctx, strings.TrimSpace(status), strings.TrimSpace(keyword), size, (page-1)*size)
	return Page[Plan]{Items: items, Total: total, Page: page, PageSize: size}, translate(err)
}
func (s *Service) UpsertUsagePrice(ctx context.Context, value UsagePrice, expected, planVersion int64) (UsagePrice, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return UsagePrice{}, err
	}
	value.PlanID = strings.TrimSpace(value.PlanID)
	value.MeterCode = strings.ToLower(strings.TrimSpace(value.MeterCode))
	value.PricingModel = strings.ToLower(strings.TrimSpace(value.PricingModel))
	if value.PlanID == "" || planVersion < 1 || value.MeterCode == "" || value.IncludedQuantity < 0 || value.UnitQuantity <= 0 || value.UnitAmountMinor < 0 || !map[string]bool{"per_unit": true, "volume": true, "graduated": true}[value.PricingModel] || !validJSON(value.TiersJSON, "[]") {
		return UsagePrice{}, apperror.Invalid("invalid usage price", nil)
	}
	now := s.now()
	if expected == 0 {
		value.ID = uuid.NewString()
		value.Audit = newAudit(actorID, now)
	} else {
		if value.ID == "" {
			return UsagePrice{}, apperror.Invalid("id is required for update", nil)
		}
		value.Version = expected + 1
		value.UpdatedAt = now
		value.UpdatedBy = actorID
	}
	value.TiersJSON = defaultJSON(value.TiersJSON, "[]")
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if _, lockErr := s.repository.LockActivePlan(ctx, tx, value.PlanID, planVersion); lockErr != nil {
			return lockErr
		}
		return s.repository.UpsertUsagePrice(ctx, tx, value, expected)
	})
	return value, translate(err)
}
func (s *Service) DeleteUsagePrice(ctx context.Context, id string, version int64, planID string, planVersion int64) error {
	if _, err := actor(ctx); err != nil {
		return err
	}
	planID = strings.TrimSpace(planID)
	if id == "" || version < 1 || planID == "" || planVersion < 1 {
		return apperror.Invalid("id, version, plan_id and plan_version are required", nil)
	}
	return translate(s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if _, err := s.repository.LockActivePlan(ctx, tx, planID, planVersion); err != nil {
			return err
		}
		return s.repository.DeleteUsagePrice(ctx, tx, id, version)
	}))
}

func (s *Service) CreateSubscription(ctx context.Context, tenantID, applicationID, planID string, planVersion int64, startsAt time.Time, externalReference string) (Subscription, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return Subscription{}, err
	}
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Subscription{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Subscription{}, err
	}
	planID = strings.TrimSpace(planID)
	if planID == "" || planVersion < 1 {
		return Subscription{}, apperror.Invalid("plan_id and positive plan_version are required", nil)
	}
	if startsAt.IsZero() {
		startsAt = s.now()
	}
	now := s.now()
	value := Subscription{ID: uuid.NewString(), TenantID: strings.TrimSpace(tenantID), ApplicationID: strings.TrimSpace(applicationID), PlanID: planID, CurrentPeriodStart: startsAt, ExternalReference: strings.TrimSpace(externalReference), Audit: newAudit(actorID, now)}
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		plan, err := s.repository.LockActivePlan(ctx, tx, planID, planVersion)
		if err != nil {
			return err
		}
		value.Status = "active"
		if plan.TrialDays > 0 {
			value.Status = "trialing"
		}
		value.CurrentPeriodEnd = addInterval(startsAt, plan.BillingInterval)
		if err := s.repository.ClaimSubscription(ctx, tx, value.TenantID, value.ApplicationID, value.ID, value.Audit); err != nil {
			return err
		}
		if err := s.repository.CreateSubscription(ctx, tx, value); err != nil {
			return err
		}
		return s.addEvent(ctx, tx, "platform.billing.subscription.changed.v1", "platform.billing.v1.SubscriptionChanged", value.ID, "subscription", value.TenantID, value.ApplicationID, actorID, now, &billingv1.SubscriptionChangedEvent{Subscription: ToProtoSubscription(value), ChangeType: "created"})
	})
	return value, translate(err)
}
func (s *Service) ChangeSubscription(ctx context.Context, tenantID, applicationID, id, planID, effectiveMode string, version, planVersion int64) (Subscription, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return Subscription{}, err
	}
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Subscription{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Subscription{}, err
	}
	if effectiveMode != "immediate" && effectiveMode != "next_period" {
		return Subscription{}, apperror.Invalid("effective_mode must be immediate or next_period", nil)
	}
	planID = strings.TrimSpace(planID)
	if planID == "" || version < 1 || planVersion < 1 {
		return Subscription{}, apperror.Invalid("plan_id, version and plan_version are required", nil)
	}
	value, err := s.repository.GetSubscription(ctx, tenantID, applicationID, id)
	if err != nil {
		return Subscription{}, translate(err)
	}
	changeType := "plan_changed"
	if effectiveMode == "next_period" {
		value.PendingPlanID = &planID
		changeAt := value.CurrentPeriodEnd
		value.PendingChangeAt = &changeAt
		changeType = "plan_change_scheduled"
	} else {
		value.PlanID = planID
		value.PendingPlanID = nil
		value.PendingChangeAt = nil
	}
	value.Version = version + 1
	value.UpdatedAt = s.now()
	value.UpdatedBy = actorID
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if _, lockErr := s.repository.LockActivePlan(ctx, tx, planID, planVersion); lockErr != nil {
			return lockErr
		}
		if err := s.repository.UpdateSubscription(ctx, tx, value, version); err != nil {
			return err
		}
		return s.addEvent(ctx, tx, "platform.billing.subscription.changed.v1", "platform.billing.v1.SubscriptionChanged", value.ID, "subscription", value.TenantID, value.ApplicationID, actorID, value.UpdatedAt, &billingv1.SubscriptionChangedEvent{Subscription: ToProtoSubscription(value), ChangeType: changeType})
	})
	return value, translate(err)
}
func (s *Service) CancelSubscription(ctx context.Context, tenantID, applicationID, id string, atPeriodEnd bool, version int64) (Subscription, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return Subscription{}, err
	}
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Subscription{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Subscription{}, err
	}
	value, err := s.repository.GetSubscription(ctx, tenantID, applicationID, id)
	if err != nil {
		return Subscription{}, translate(err)
	}
	if version < 1 {
		return Subscription{}, apperror.Invalid("positive version is required", nil)
	}
	now := s.now()
	value.CancelAtPeriodEnd = atPeriodEnd
	if !atPeriodEnd {
		value.Status = "canceled"
		value.CanceledAt = &now
		value.PendingPlanID = nil
		value.PendingChangeAt = nil
	}
	value.Version = version + 1
	value.UpdatedAt = now
	value.UpdatedBy = actorID
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if err := s.repository.UpdateSubscription(ctx, tx, value, version); err != nil {
			return err
		}
		if !atPeriodEnd {
			if err := s.repository.ReleaseSubscriptionClaim(ctx, tx, value.TenantID, value.ApplicationID, value.ID); err != nil {
				return err
			}
		}
		changeType := "canceled"
		if atPeriodEnd {
			changeType = "cancellation_scheduled"
		}
		return s.addEvent(ctx, tx, "platform.billing.subscription.changed.v1", "platform.billing.v1.SubscriptionChanged", value.ID, "subscription", value.TenantID, value.ApplicationID, actorID, now, &billingv1.SubscriptionChangedEvent{Subscription: ToProtoSubscription(value), ChangeType: changeType})
	})
	return value, translate(err)
}

func (s *Service) ApplyDueSubscriptionTransitions(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	now := s.now()
	values, err := s.repository.ListDueSubscriptions(ctx, now, limit)
	if err != nil {
		return 0, translate(err)
	}
	applied := 0
	for _, value := range values {
		current := value
		expected := current.Version
		changeType := "plan_change_applied"
		if current.CancelAtPeriodEnd && !current.CurrentPeriodEnd.After(now) {
			current.Status = "canceled"
			current.CanceledAt = &now
			current.CancelAtPeriodEnd = false
			current.PendingPlanID = nil
			current.PendingChangeAt = nil
			changeType = "canceled"
		} else if current.PendingPlanID != nil && current.PendingChangeAt != nil && !current.PendingChangeAt.After(now) {
			plan, planErr := s.repository.GetPlan(ctx, *current.PendingPlanID, "")
			if planErr != nil {
				return applied, translate(planErr)
			}
			current.PlanID = plan.ID
			current.CurrentPeriodStart = current.CurrentPeriodEnd
			current.CurrentPeriodEnd = addInterval(current.CurrentPeriodStart, plan.BillingInterval)
			current.PendingPlanID = nil
			current.PendingChangeAt = nil
		} else {
			continue
		}
		current.Version, current.UpdatedAt, current.UpdatedBy = expected+1, now, "billing-service:scheduler"
		err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
			if updateErr := s.repository.UpdateSubscription(ctx, tx, current, expected); updateErr != nil {
				return updateErr
			}
			if current.Status == "canceled" {
				if releaseErr := s.repository.ReleaseSubscriptionClaim(ctx, tx, current.TenantID, current.ApplicationID, current.ID); releaseErr != nil {
					return releaseErr
				}
			}
			return s.addEvent(ctx, tx, "platform.billing.subscription.changed.v1", "platform.billing.v1.SubscriptionChanged", current.ID, "subscription", current.TenantID, current.ApplicationID, "billing-service:scheduler", now, &billingv1.SubscriptionChangedEvent{Subscription: ToProtoSubscription(current), ChangeType: changeType})
		})
		if errors.Is(err, ErrStaleVersion) {
			continue
		}
		if err != nil {
			return applied, translate(err)
		}
		applied++
	}
	return applied, nil
}
func (s *Service) GetSubscription(ctx context.Context, tenantID, applicationID, id string) (Subscription, Plan, error) {
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Subscription{}, Plan{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Subscription{}, Plan{}, err
	}
	value, err := s.repository.GetSubscription(ctx, tenantID, applicationID, id)
	if err != nil {
		return Subscription{}, Plan{}, translate(err)
	}
	plan, err := s.repository.GetPlan(ctx, value.PlanID, "")
	return value, plan, translate(err)
}
func (s *Service) ListSubscriptions(ctx context.Context, tenantID, applicationID, status string, page, size int) (Page[Subscription], error) {
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Page[Subscription]{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Page[Subscription]{}, err
	}
	page, size, err := pagination(page, size)
	if err != nil {
		return Page[Subscription]{}, err
	}
	items, total, err := s.repository.ListSubscriptions(ctx, tenantID, applicationID, status, size, (page-1)*size)
	return Page[Subscription]{Items: items, Total: total, Page: page, PageSize: size}, translate(err)
}

func (s *Service) PreviewInvoice(ctx context.Context, tenantID, applicationID, subscriptionID string, start, end time.Time) (InvoicePreview, error) {
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return InvoicePreview{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return InvoicePreview{}, err
	}
	subscription, err := s.repository.GetSubscription(ctx, tenantID, applicationID, subscriptionID)
	if err != nil {
		return InvoicePreview{}, translate(err)
	}
	plan, err := s.repository.GetPlan(ctx, subscription.PlanID, "")
	if err != nil {
		return InvoicePreview{}, translate(err)
	}
	prices, err := s.repository.ListUsagePrices(ctx, plan.ID)
	if err != nil {
		return InvoicePreview{}, translate(err)
	}
	if start.IsZero() {
		start = subscription.CurrentPeriodStart
	}
	if end.IsZero() {
		end = subscription.CurrentPeriodEnd
	}
	if !end.After(start) || end.Sub(start) > 370*24*time.Hour {
		return InvoicePreview{}, apperror.Invalid("valid invoice period of at most 370 days is required", nil)
	}
	now := s.now()
	invoiceID := uuid.NewString()
	invoice := Invoice{ID: invoiceID, TenantID: tenantID, ApplicationID: applicationID, SubscriptionID: subscription.ID, Currency: plan.Currency, Status: "draft", PeriodStart: start, PeriodEnd: end, Audit: newAudit("preview", now)}
	lines := []InvoiceLine{{ID: uuid.NewString(), InvoiceID: invoiceID, Type: "base", Description: plan.Name, Quantity: 1, UnitQuantity: 1, UnitAmountMinor: plan.BaseAmountMinor, AmountMinor: plan.BaseAmountMinor, MetadataJSON: "{}", Audit: newAudit("preview", now)}}
	subtotal := plan.BaseAmountMinor
	for _, price := range prices {
		if s.usage == nil {
			return InvoicePreview{}, apperror.Unavailable("metering client is unavailable", nil)
		}
		quantity, err := s.usage.Total(ctx, tenantID, applicationID, price.MeterCode, start, end)
		if err != nil {
			return InvoicePreview{}, apperror.Unavailable("query metering usage", err)
		}
		billable := max(int64(0), quantity-price.IncludedQuantity)
		amount, err := usageAmount(billable, price.UnitQuantity, price.UnitAmountMinor)
		if err != nil {
			return InvoicePreview{}, err
		}
		lines = append(lines, InvoiceLine{ID: uuid.NewString(), InvoiceID: invoiceID, Type: "usage", Description: price.MeterCode, MeterCode: price.MeterCode, Quantity: quantity, UnitQuantity: price.UnitQuantity, UnitAmountMinor: price.UnitAmountMinor, AmountMinor: amount, MetadataJSON: "{}", Audit: newAudit("preview", now)})
		if subtotal > math.MaxInt64-amount {
			return InvoicePreview{}, apperror.Invalid("invoice amount overflow", nil)
		}
		subtotal += amount
	}
	invoice.SubtotalMinor = subtotal
	invoice.TotalMinor = subtotal
	return InvoicePreview{Invoice: invoice, Lines: lines}, nil
}
func (s *Service) GenerateInvoice(ctx context.Context, tenantID, applicationID, subscriptionID string, start, end time.Time, key string) (InvoicePreview, bool, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return InvoicePreview{}, false, err
	}
	if strings.TrimSpace(key) == "" {
		return InvoicePreview{}, false, apperror.Invalid("idempotency_key is required", nil)
	}
	preview, err := s.PreviewInvoice(ctx, tenantID, applicationID, subscriptionID, start, end)
	if err != nil {
		return InvoicePreview{}, false, err
	}
	now := s.now()
	preview.Invoice.Number = invoiceNumber(now, preview.Invoice.ID)
	preview.Invoice.CreatedBy = actorID
	preview.Invoice.UpdatedBy = actorID
	for i := range preview.Lines {
		preview.Lines[i].CreatedBy = actorID
		preview.Lines[i].UpdatedBy = actorID
	}
	requestHash := hashParts(tenantID, applicationID, subscriptionID, preview.Invoice.PeriodStart.Format(time.RFC3339Nano), preview.Invoice.PeriodEnd.Format(time.RFC3339Nano))
	duplicateID := ""
	created := false
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		claimedID, claimed, err := s.repository.ClaimInvoice(ctx, tx, key, preview.Invoice.ID, tenantID, applicationID, requestHash, preview.Invoice.Audit)
		if err != nil {
			return err
		}
		if !claimed {
			duplicateID = claimedID
			return nil
		}
		created = true
		if err := s.repository.CreateInvoice(ctx, tx, preview.Invoice, preview.Lines); err != nil {
			return err
		}
		return s.addEvent(ctx, tx, "platform.billing.invoice.changed.v1", "platform.billing.v1.InvoiceChanged", preview.Invoice.ID, "invoice", tenantID, preview.Invoice.ApplicationID, actorID, now, &billingv1.InvoiceChangedEvent{Invoice: ToProtoInvoice(preview.Invoice), Lines: ToProtoInvoiceLines(preview.Lines), ChangeType: "generated"})
	})
	if err != nil {
		return InvoicePreview{}, false, translate(err)
	}
	if !created {
		invoice, lines, err := s.repository.GetInvoice(ctx, tenantID, applicationID, duplicateID)
		return InvoicePreview{Invoice: invoice, Lines: lines}, true, translate(err)
	}
	return preview, false, nil
}
func (s *Service) FinalizeInvoice(ctx context.Context, tenantID, applicationID, id string, dueAt time.Time, version int64) (Invoice, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return Invoice{}, err
	}
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Invoice{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Invoice{}, err
	}
	value, _, err := s.repository.GetInvoice(ctx, tenantID, applicationID, id)
	if err != nil {
		return Invoice{}, translate(err)
	}
	if value.Status != "draft" || version < 1 {
		return Invoice{}, apperror.Conflict("only a current draft invoice can be finalized", nil)
	}
	now := s.now()
	if dueAt.IsZero() {
		dueAt = now.Add(7 * 24 * time.Hour)
	}
	value.Status = "open"
	value.DueAt = &dueAt
	value.FinalizedAt = &now
	value.Version = version + 1
	value.UpdatedAt = now
	value.UpdatedBy = actorID
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error { return s.repository.UpdateInvoice(ctx, tx, value, version) })
	return value, translate(err)
}
func (s *Service) VoidInvoice(ctx context.Context, tenantID, applicationID, id, reason string, version int64) (Invoice, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return Invoice{}, err
	}
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Invoice{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Invoice{}, err
	}
	if strings.TrimSpace(reason) == "" {
		return Invoice{}, apperror.Invalid("void reason is required", nil)
	}
	value, _, err := s.repository.GetInvoice(ctx, tenantID, applicationID, id)
	if err != nil {
		return Invoice{}, translate(err)
	}
	if value.Status == "paid" || value.PaidMinor > 0 || version < 1 {
		return Invoice{}, apperror.Conflict("paid invoice cannot be voided", nil)
	}
	value.Status = "void"
	value.Version = version + 1
	value.UpdatedAt = s.now()
	value.UpdatedBy = actorID
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error { return s.repository.UpdateInvoice(ctx, tx, value, version) })
	return value, translate(err)
}
func (s *Service) GetInvoice(ctx context.Context, tenantID, applicationID, id string) (Invoice, []InvoiceLine, error) {
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Invoice{}, nil, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Invoice{}, nil, err
	}
	v, lines, err := s.repository.GetInvoice(ctx, tenantID, applicationID, id)
	return v, lines, translate(err)
}
func (s *Service) ListInvoices(ctx context.Context, tenantID, applicationID, status string, from, to time.Time, page, size int) (Page[Invoice], error) {
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Page[Invoice]{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Page[Invoice]{}, err
	}
	page, size, err := pagination(page, size)
	if err != nil {
		return Page[Invoice]{}, err
	}
	items, total, err := s.repository.ListInvoices(ctx, tenantID, applicationID, status, from, to, size, (page-1)*size)
	return Page[Invoice]{Items: items, Total: total, Page: page, PageSize: size}, translate(err)
}

func (s *Service) ListPayableInvoices(ctx context.Context, tenantID, applicationID string, page, size int) (Page[Invoice], error) {
	return s.ListInvoices(ctx, tenantID, applicationID, "open", time.Time{}, time.Time{}, page, size)
}

func (s *Service) CreatePaymentAttempt(ctx context.Context, tenantID, applicationID, invoiceID, provider, paymentMethodReference, key string) (PaymentAttempt, bool, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return PaymentAttempt{}, false, err
	}
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return PaymentAttempt{}, false, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return PaymentAttempt{}, false, err
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	paymentMethodReference = strings.TrimSpace(paymentMethodReference)
	key = strings.TrimSpace(key)
	if invoiceID == "" || provider == "" || paymentMethodReference == "" || key == "" {
		return PaymentAttempt{}, false, apperror.Invalid("invoice_id, provider, payment_method_reference and idempotency_key are required", nil)
	}
	requestHash := hashParts(tenantID, applicationID, invoiceID, provider, paymentMethodReference)
	if existing, findErr := s.repository.GetPaymentByKey(ctx, key); findErr == nil {
		if existing.TenantID != tenantID || existing.ApplicationID != applicationID || existing.RequestHash != requestHash {
			return PaymentAttempt{}, false, apperror.Conflict("idempotency key belongs to a different payment request", nil)
		}
		if existing.Status != "pending" && existing.Status != "requires_action" {
			return existing, true, nil
		}
		if s.gateway == nil {
			return existing, true, apperror.Unavailable("payment gateway is unavailable", nil)
		}
		result, gatewayErr := s.gateway.Create(ctx, PaymentCommand{AttemptID: existing.ID, TenantID: existing.TenantID, ApplicationID: existing.ApplicationID, InvoiceID: existing.InvoiceID, Provider: existing.Provider, PaymentMethodReference: paymentMethodReference, Currency: existing.Currency, AmountMinor: existing.AmountMinor})
		if gatewayErr != nil {
			return existing, true, apperror.Unavailable("create provider payment", gatewayErr)
		}
		updated, _, _, applyErr := s.ApplyPaymentResult(ctx, existing.ID, result.ProviderPaymentID, result.ProviderEventID, result.Status, result.FailureCode, result.FailureMessage, result.ProcessedAt)
		return updated, true, applyErr
	} else if !errors.Is(findErr, ErrNotFound) {
		return PaymentAttempt{}, false, translate(findErr)
	}
	invoice, _, err := s.repository.GetInvoice(ctx, tenantID, applicationID, invoiceID)
	if err != nil {
		return PaymentAttempt{}, false, translate(err)
	}
	outstanding := invoice.TotalMinor - invoice.PaidMinor
	if invoice.Status != "open" || outstanding <= 0 {
		return PaymentAttempt{}, false, apperror.Conflict("invoice is not payable", nil)
	}
	now := s.now()
	value := PaymentAttempt{ID: uuid.NewString(), InvoiceID: invoice.ID, TenantID: tenantID, ApplicationID: applicationID, Provider: provider, IdempotencyKey: key, RequestHash: requestHash, Currency: invoice.Currency, AmountMinor: outstanding, Status: "pending", Audit: newAudit(actorID, now)}
	existingID := ""
	created := false
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		id, claimed, claimErr := s.repository.ClaimPayment(ctx, tx, value)
		if claimErr != nil {
			return claimErr
		}
		existingID, created = id, claimed
		if !claimed {
			return nil
		}
		return s.addEvent(ctx, tx, "platform.billing.payment.changed.v1", "platform.billing.v1.PaymentChanged", value.ID, "payment_attempt", tenantID, value.ApplicationID, actorID, now, &billingv1.PaymentChangedEvent{PaymentAttempt: ToProtoPayment(value), Invoice: ToProtoInvoice(invoice), ChangeType: "requested"})
	})
	if err != nil {
		return PaymentAttempt{}, false, translate(err)
	}
	if !created {
		existing, getErr := s.repository.GetPayment(ctx, tenantID, applicationID, existingID)
		if getErr != nil {
			return PaymentAttempt{}, true, translate(getErr)
		}
		value = existing
	}
	if value.Status != "pending" && value.Status != "requires_action" {
		return value, !created, nil
	}
	if s.gateway == nil {
		return value, !created, apperror.Unavailable("payment gateway is unavailable", nil)
	}
	result, gatewayErr := s.gateway.Create(ctx, PaymentCommand{AttemptID: value.ID, TenantID: value.TenantID, ApplicationID: value.ApplicationID, InvoiceID: value.InvoiceID, Provider: value.Provider, PaymentMethodReference: paymentMethodReference, Currency: value.Currency, AmountMinor: value.AmountMinor})
	if gatewayErr != nil {
		return value, !created, apperror.Unavailable("create provider payment", gatewayErr)
	}
	updated, _, _, applyErr := s.ApplyPaymentResult(ctx, value.ID, result.ProviderPaymentID, result.ProviderEventID, result.Status, result.FailureCode, result.FailureMessage, result.ProcessedAt)
	return updated, !created, applyErr
}

func (s *Service) ListPayments(ctx context.Context, tenantID, applicationID, status string, page, size int) (Page[PaymentAttempt], error) {
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Page[PaymentAttempt]{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Page[PaymentAttempt]{}, err
	}
	page, size, err := pagination(page, size)
	if err != nil {
		return Page[PaymentAttempt]{}, err
	}
	items, total, err := s.repository.ListPayments(ctx, tenantID, applicationID, strings.TrimSpace(status), size, (page-1)*size)
	return Page[PaymentAttempt]{Items: items, Total: total, Page: page, PageSize: size}, translate(err)
}

func (s *Service) GetPayment(ctx context.Context, tenantID, applicationID, id string) (PaymentAttempt, error) {
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return PaymentAttempt{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return PaymentAttempt{}, err
	}
	value, err := s.repository.GetPayment(ctx, tenantID, applicationID, strings.TrimSpace(id))
	return value, translate(err)
}

func (s *Service) ApplyPaymentResult(ctx context.Context, paymentID, providerPaymentID, providerEventID, status, failureCode, failureMessage string, processedAt time.Time) (PaymentAttempt, Invoice, bool, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return PaymentAttempt{}, Invoice{}, false, err
	}
	value, err := s.repository.GetPayment(ctx, "", "", strings.TrimSpace(paymentID))
	if err != nil {
		return PaymentAttempt{}, Invoice{}, false, translate(err)
	}
	if err := authorizeTenant(ctx, value.TenantID); err != nil {
		return PaymentAttempt{}, Invoice{}, false, err
	}
	if err := s.verifyApplication(ctx, value.TenantID, value.ApplicationID); err != nil {
		return PaymentAttempt{}, Invoice{}, false, err
	}
	status = strings.ToLower(strings.TrimSpace(status))
	providerEventID = strings.TrimSpace(providerEventID)
	if providerEventID == "" || !map[string]bool{"pending": true, "requires_action": true, "succeeded": true, "failed": true, "canceled": true}[status] {
		return PaymentAttempt{}, Invoice{}, false, apperror.Invalid("provider_event_id and a valid payment status are required", nil)
	}
	if !validPaymentTransition(value.Status, status) {
		return PaymentAttempt{}, Invoice{}, false, apperror.Conflict("payment status transition is not allowed", nil)
	}
	invoice, _, err := s.repository.GetInvoice(ctx, value.TenantID, value.ApplicationID, value.InvoiceID)
	if err != nil {
		return PaymentAttempt{}, Invoice{}, false, translate(err)
	}
	if processedAt.IsZero() {
		processedAt = s.now()
	}
	now := s.now()
	duplicate := false
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		claimed, claimErr := s.repository.ClaimProviderEvent(ctx, tx, value.Provider, providerEventID, value.ID, newAudit(actorID, now))
		if claimErr != nil {
			return claimErr
		}
		if !claimed {
			duplicate = true
			return nil
		}
		paymentVersion := value.Version
		value.ProviderPaymentID = strings.TrimSpace(providerPaymentID)
		value.Status, value.FailureCode, value.FailureMessage = status, strings.TrimSpace(failureCode), strings.TrimSpace(failureMessage)
		value.ProcessedAt, value.Version, value.UpdatedAt, value.UpdatedBy = &processedAt, paymentVersion+1, now, actorID
		if err := s.repository.UpdatePayment(ctx, tx, value, paymentVersion); err != nil {
			return err
		}
		if status == "succeeded" {
			if value.AmountMinor > invoice.TotalMinor-invoice.PaidMinor {
				return ErrConflict
			}
			invoiceVersion := invoice.Version
			invoice.PaidMinor += value.AmountMinor
			if invoice.PaidMinor == invoice.TotalMinor {
				invoice.Status, invoice.PaidAt = "paid", &processedAt
			}
			invoice.Version, invoice.UpdatedAt, invoice.UpdatedBy = invoiceVersion+1, now, actorID
			if err := s.repository.UpdateInvoice(ctx, tx, invoice, invoiceVersion); err != nil {
				return err
			}
		}
		return s.addEvent(ctx, tx, "platform.billing.payment.changed.v1", "platform.billing.v1.PaymentChanged", value.ID, "payment_attempt", value.TenantID, value.ApplicationID, actorID, now, &billingv1.PaymentChangedEvent{PaymentAttempt: ToProtoPayment(value), Invoice: ToProtoInvoice(invoice), ChangeType: status})
	})
	return value, invoice, duplicate, translate(err)
}

func validPaymentTransition(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case "pending", "requires_action":
		return map[string]bool{"pending": true, "requires_action": true, "succeeded": true, "failed": true, "canceled": true}[to]
	default:
		return false
	}
}

func (s *Service) RecordRefund(ctx context.Context, tenantID, applicationID, paymentID, providerRefundID, key string, amount int64, reason, status string) (Refund, Invoice, bool, error) {
	actorID, err := actor(ctx)
	if err != nil {
		return Refund{}, Invoice{}, false, err
	}
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Refund{}, Invoice{}, false, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Refund{}, Invoice{}, false, err
	}
	payment, err := s.repository.GetPayment(ctx, tenantID, applicationID, paymentID)
	if err != nil {
		return Refund{}, Invoice{}, false, translate(err)
	}
	if payment.Status != "succeeded" || amount <= 0 || strings.TrimSpace(key) == "" || strings.TrimSpace(reason) == "" {
		return Refund{}, Invoice{}, false, apperror.Invalid("a successful payment, positive amount, reason and idempotency_key are required", nil)
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if !map[string]bool{"pending": true, "succeeded": true, "failed": true}[status] {
		return Refund{}, Invoice{}, false, apperror.Invalid("invalid refund status", nil)
	}
	invoice, _, err := s.repository.GetInvoice(ctx, tenantID, applicationID, payment.InvoiceID)
	if err != nil {
		return Refund{}, Invoice{}, false, translate(err)
	}
	if status == "succeeded" && amount > invoice.PaidMinor-invoice.RefundedMinor {
		return Refund{}, Invoice{}, false, apperror.Conflict("refund exceeds refundable amount", nil)
	}
	now := s.now()
	value := Refund{ID: uuid.NewString(), PaymentAttemptID: payment.ID, InvoiceID: invoice.ID, TenantID: tenantID, ApplicationID: applicationID, ProviderRefundID: strings.TrimSpace(providerRefundID), IdempotencyKey: strings.TrimSpace(key), RequestHash: hashParts(tenantID, applicationID, paymentID, fmt.Sprint(amount), strings.TrimSpace(reason)), AmountMinor: amount, Reason: strings.TrimSpace(reason), Status: status, Audit: newAudit(actorID, now)}
	existingID, created := "", false
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		id, claimed, claimErr := s.repository.ClaimRefund(ctx, tx, value)
		if claimErr != nil {
			return claimErr
		}
		existingID, created = id, claimed
		if !claimed {
			return nil
		}
		if status == "succeeded" {
			invoiceVersion := invoice.Version
			invoice.RefundedMinor += amount
			if invoice.RefundedMinor == invoice.PaidMinor {
				invoice.Status = "refunded"
			}
			invoice.Version, invoice.UpdatedAt, invoice.UpdatedBy = invoiceVersion+1, now, actorID
			if err := s.repository.UpdateInvoice(ctx, tx, invoice, invoiceVersion); err != nil {
				return err
			}
		}
		return s.addEvent(ctx, tx, "platform.billing.refund.changed.v1", "platform.billing.v1.RefundChanged", value.ID, "refund", tenantID, value.ApplicationID, actorID, now, &billingv1.RefundChangedEvent{Refund: ToProtoRefund(value), Invoice: ToProtoInvoice(invoice), ChangeType: status})
	})
	if err != nil {
		return Refund{}, Invoice{}, false, translate(err)
	}
	if !created {
		existing, getErr := s.repository.GetRefund(ctx, existingID)
		return existing, invoice, true, translate(getErr)
	}
	return value, invoice, false, nil
}

func (s *Service) ListRefunds(ctx context.Context, tenantID, applicationID, status string, page, size int) (Page[Refund], error) {
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Page[Refund]{}, err
	}
	if err := s.verifyApplication(ctx, tenantID, applicationID); err != nil {
		return Page[Refund]{}, err
	}
	page, size, err := pagination(page, size)
	if err != nil {
		return Page[Refund]{}, err
	}
	items, total, err := s.repository.ListRefunds(ctx, tenantID, applicationID, strings.TrimSpace(status), size, (page-1)*size)
	return Page[Refund]{Items: items, Total: total, Page: page, PageSize: size}, translate(err)
}

func (s *Service) ReconcilePayment(ctx context.Context, provider string, from, to time.Time, cursor string, limit uint32) ([]ReconciliationMismatch, string, error) {
	if _, err := actor(ctx); err != nil {
		return nil, "", err
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" || from.IsZero() || to.IsZero() || !to.After(from) || limit == 0 || limit > 500 {
		return nil, "", apperror.Invalid("provider, valid time range, and limit 1..500 are required", nil)
	}
	if s.gateway == nil {
		return nil, "", apperror.Unavailable("payment gateway is unavailable", nil)
	}
	values, next, err := s.gateway.Reconcile(ctx, provider, from, to, strings.TrimSpace(cursor), limit)
	if err != nil {
		return nil, "", apperror.Unavailable("reconcile provider payments", err)
	}
	return values, next, nil
}

func (s *Service) addEvent(ctx context.Context, tx *sqlx.Tx, subject, eventType, aggregateID, aggregateType, tenantID, applicationID, actorID string, at time.Time, payload proto.Message) error {
	envelope, err := platformevents.NewEnvelope(platformevents.Metadata{EventID: uuid.NewString(), EventType: eventType, AggregateID: aggregateID, AggregateType: aggregateType, TenantID: tenantID, ApplicationID: applicationID, SchemaVersion: 1, ActorID: actorID, OccurredAt: at}, payload)
	if err != nil {
		return err
	}
	encoded, err := proto.Marshal(envelope)
	if err != nil {
		return err
	}
	return s.repository.AddOutbox(ctx, tx, OutboxEvent{ID: envelope.GetEventId(), Subject: subject, Envelope: encoded, AvailableAt: at, CreatedAt: at, UpdatedAt: at, CreatedBy: actorID, UpdatedBy: actorID})
}
func actor(ctx context.Context) (string, error) {
	v, ok := platformprincipal.FromContext(ctx)
	if !ok {
		return "", apperror.Unauthorized("authenticated actor is required")
	}
	return v.ID, nil
}
func authorizeTenant(ctx context.Context, tenantID string) error {
	v, ok := platformprincipal.FromContext(ctx)
	if !ok {
		return apperror.Unauthorized("authenticated actor is required")
	}
	if v.Type == platformprincipal.TypeServiceAccount || v.Type == platformprincipal.TypeSystem {
		return nil
	}
	if v.TenantID == "" || v.TenantID != strings.TrimSpace(tenantID) {
		return apperror.Forbidden("tenant access denied")
	}
	return nil
}

func (s *Service) verifyApplication(ctx context.Context, tenantID, applicationID string) error {
	tenantID = strings.TrimSpace(tenantID)
	applicationID = strings.TrimSpace(applicationID)
	if tenantID == "" || applicationID == "" {
		return apperror.Invalid("tenant_id and application_id are required", nil)
	}
	err := s.applications.Verify(ctx, tenantID, applicationID)
	if errors.Is(err, appaccess.ErrNotGranted) {
		return apperror.Forbidden("application access denied")
	}
	if err != nil {
		return apperror.Unavailable("application authorization is unavailable", err)
	}
	return nil
}
func newAudit(actorID string, now time.Time) Audit {
	return Audit{Version: 1, CreatedAt: now, UpdatedAt: now, CreatedBy: actorID, UpdatedBy: actorID}
}
func validInterval(v string) bool { return v == "month" || v == "year" }
func addInterval(v time.Time, interval string) time.Time {
	if interval == "year" {
		return addMonthsClamped(v, 12)
	}
	return addMonthsClamped(v, 1)
}

// addMonthsClamped keeps a subscription anchored to the same calendar day and
// uses the destination month's last day when that day does not exist.
func addMonthsClamped(v time.Time, months int) time.Time {
	first := time.Date(v.Year(), v.Month(), 1, v.Hour(), v.Minute(), v.Second(), v.Nanosecond(), v.Location())
	target := first.AddDate(0, months, 0)
	lastDay := time.Date(target.Year(), target.Month()+1, 0, v.Hour(), v.Minute(), v.Second(), v.Nanosecond(), v.Location()).Day()
	return time.Date(target.Year(), target.Month(), min(v.Day(), lastDay), v.Hour(), v.Minute(), v.Second(), v.Nanosecond(), v.Location())
}
func defaultJSON(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
func validJSON(v, fallback string) bool { return json.Valid([]byte(defaultJSON(v, fallback))) }
func compactJSON(value string) string {
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return value
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		return value
	}
	return string(canonical)
}
func usageAmount(quantity, unit, amount int64) (int64, error) {
	if quantity <= 0 {
		return 0, nil
	}
	units := (quantity-1)/unit + 1
	if amount > 0 && units > math.MaxInt64/amount {
		return 0, apperror.Invalid("usage amount overflow", nil)
	}
	return units * amount, nil
}
func invoiceNumber(at time.Time, id string) string {
	return fmt.Sprintf("INV-%s-%s", at.In(time.FixedZone("UTC+8", 8*60*60)).Format("20060102"), strings.ToUpper(strings.ReplaceAll(id, "-", "")[:12]))
}
func hashParts(values ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return hex.EncodeToString(hash[:])
}
func pagination(page, size int) (int, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		return 0, 0, apperror.Invalid("page_size must not exceed 100", nil)
	}
	return page, size, nil
}
func translate(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return apperror.NotFound("billing resource not found")
	case errors.Is(err, ErrStaleVersion):
		return apperror.Conflict("resource version is stale", err)
	case errors.Is(err, ErrConflict):
		return apperror.Conflict("billing resource conflicts with current state", err)
	case strings.Contains(strings.ToLower(err.Error()), "unique"), strings.Contains(strings.ToLower(err.Error()), "duplicate"):
		return apperror.Conflict("billing resource already exists", err)
	default:
		return apperror.Internal(err)
	}
}
