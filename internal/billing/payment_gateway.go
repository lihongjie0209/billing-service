package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lihongjie0209/billing-service/internal/idempotency"
	"github.com/lihongjie0209/billing-service/internal/outbound"
)

type PaymentCommand struct {
	AttemptID              string `json:"attempt_id"`
	TenantID               string `json:"tenant_id"`
	ApplicationID          string `json:"application_id"`
	InvoiceID              string `json:"invoice_id"`
	Provider               string `json:"provider"`
	PaymentMethodReference string `json:"payment_method_reference"`
	Currency               string `json:"currency"`
	AmountMinor            int64  `json:"amount_minor"`
}
type PaymentGatewayResult struct {
	ProviderPaymentID string    `json:"provider_payment_id"`
	ProviderEventID   string    `json:"provider_event_id"`
	Status            string    `json:"status"`
	FailureCode       string    `json:"failure_code"`
	FailureMessage    string    `json:"failure_message"`
	ProcessedAt       time.Time `json:"processed_at"`
}
type ReconciliationMismatch struct {
	ProviderPaymentID   string
	PaymentAttemptID    string
	Reason              string
	ProviderAmountMinor int64
	LocalAmountMinor    int64
}
type PaymentGateway interface {
	Create(context.Context, PaymentCommand) (PaymentGatewayResult, error)
	Reconcile(context.Context, string, time.Time, time.Time, string, uint32) ([]ReconciliationMismatch, string, error)
}

type HTTPPaymentGateway struct{ client *outbound.HTTPClient }

func NewPaymentGateway(registry *outbound.Registry) PaymentGateway {
	client, _ := registry.HTTP("payment_gateway")
	return &HTTPPaymentGateway{client: client}
}

type gatewayEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Body    T      `json:"body"`
}

func (g *HTTPPaymentGateway) Create(ctx context.Context, command PaymentCommand) (PaymentGatewayResult, error) {
	if g.client == nil {
		return PaymentGatewayResult{}, errors.New("payment_gateway upstream is not configured")
	}
	body, err := json.Marshal(command)
	if err != nil {
		return PaymentGatewayResult{}, fmt.Errorf("encode payment command: %w", err)
	}
	ctx = idempotency.WithContext(ctx, command.AttemptID)
	response, err := g.client.Do(ctx, http.MethodPost, "payments/create", body, http.Header{"Idempotency-Key": []string{command.AttemptID}})
	if err != nil {
		return PaymentGatewayResult{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.CopyN(io.Discard, response.Body, 32<<10)
		return PaymentGatewayResult{}, fmt.Errorf("payment gateway status %d", response.StatusCode)
	}
	var envelope gatewayEnvelope[PaymentGatewayResult]
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return PaymentGatewayResult{}, fmt.Errorf("decode payment gateway response: %w", err)
	}
	if envelope.Code != 0 {
		return PaymentGatewayResult{}, fmt.Errorf("payment gateway error %d: %s", envelope.Code, envelope.Message)
	}
	return envelope.Body, nil
}
func (g *HTTPPaymentGateway) Reconcile(ctx context.Context, provider string, from, to time.Time, cursor string, limit uint32) ([]ReconciliationMismatch, string, error) {
	if g.client == nil {
		return nil, "", errors.New("payment_gateway upstream is not configured")
	}
	request := struct {
		Provider string    `json:"provider"`
		From     time.Time `json:"from"`
		To       time.Time `json:"to"`
		Cursor   string    `json:"cursor"`
		Limit    uint32    `json:"limit"`
	}{provider, from, to, cursor, limit}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, "", fmt.Errorf("encode reconciliation request: %w", err)
	}
	ctx = idempotency.WithContext(ctx, hashParts(provider, from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano), cursor, fmt.Sprint(limit)))
	response, err := g.client.Do(ctx, http.MethodPost, "payments/reconcile", body, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("payment gateway status %d", response.StatusCode)
	}
	var envelope gatewayEnvelope[struct {
		Mismatches []ReconciliationMismatch `json:"mismatches"`
		NextCursor string                   `json:"next_cursor"`
	}]
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return nil, "", fmt.Errorf("decode reconciliation response: %w", err)
	}
	if envelope.Code != 0 {
		return nil, "", fmt.Errorf("payment gateway error %d: %s", envelope.Code, envelope.Message)
	}
	return envelope.Body.Mismatches, envelope.Body.NextCursor, nil
}
