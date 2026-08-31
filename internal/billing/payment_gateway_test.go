package billing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lihongjie0209/billing-service/internal/config"
	"github.com/lihongjie0209/billing-service/internal/outbound"
)

func TestHTTPPaymentGatewayCreateUsesAttemptAsIdempotencyKey(t *testing.T) {
	t.Parallel()
	var received PaymentCommand
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/payments/create" || r.Header.Get("Idempotency-Key") != "attempt-1" {
			t.Errorf("path=%s idempotency=%s", r.URL.Path, r.Header.Get("Idempotency-Key"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		_ = json.NewEncoder(w).Encode(gatewayEnvelope[PaymentGatewayResult]{Body: PaymentGatewayResult{ProviderPaymentID: "provider-1", ProviderEventID: "event-1", Status: "succeeded"}})
	}))
	defer server.Close()
	client, err := outbound.NewHTTPClient("payment_gateway", config.HTTPUpstream{BaseURL: server.URL + "/", Timeout: time.Second, Retry: config.Retry{MaxAttempts: 1, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	gateway := &HTTPPaymentGateway{client: client}
	result, err := gateway.Create(t.Context(), PaymentCommand{AttemptID: "attempt-1", PaymentMethodReference: "secret-reference", AmountMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderPaymentID != "provider-1" || received.PaymentMethodReference != "secret-reference" {
		t.Fatalf("result=%+v command=%+v", result, received)
	}
}
