package httptransport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lihongjie0209/billing-service/internal/billing"
)

func TestPaymentAndRefundBodiesDoNotExposeIdempotencyState(t *testing.T) {
	t.Parallel()
	const secret = "one-time-internal-secret"
	values := []any{
		paymentAttemptBody(billing.PaymentAttempt{ID: "payment-1", IdempotencyKey: secret, RequestHash: secret}),
		refundBody(billing.Refund{ID: "refund-1", IdempotencyKey: secret, RequestHash: secret}),
	}
	for _, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		body := string(encoded)
		for _, forbidden := range []string{"idempotency_key", "request_hash", secret} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("response exposed internal idempotency state %q: %s", forbidden, body)
			}
		}
	}
}

func TestPlanAndInvoiceLineBodiesNormalizeStoredJSON(t *testing.T) {
	t.Parallel()
	plan := planBody(billing.Plan{EntitlementsJSON: `{"seats":10}`})
	line := invoiceLineBody(billing.InvoiceLine{MetadataJSON: `{"source":"usage"}`})
	if string(plan.EntitlementsJSON) != `{"seats":10}` || string(line.MetadataJSON) != `{"source":"usage"}` {
		t.Fatalf("normalized JSON = %s / %s", plan.EntitlementsJSON, line.MetadataJSON)
	}
}
