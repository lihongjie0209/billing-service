package httptransport

import (
	"encoding/json"
	"testing"

	"github.com/lihongjie0209/billing-service/internal/billing"
)

func TestBillingTransportEmitsStructuredJSON(t *testing.T) {
	t.Parallel()
	value := struct {
		Plan  PlanBody          `json:"plan"`
		Price UsagePriceBody    `json:"price"`
		Lines []InvoiceLineBody `json:"lines"`
	}{
		Plan:  planBody(billing.Plan{EntitlementsJSON: `{"seats":10}`}),
		Price: usagePriceBody(billing.UsagePrice{TiersJSON: `[{"up_to":100}]`}),
		Lines: mapBodies([]billing.InvoiceLine{{MetadataJSON: `{"source":"usage"}`}}, invoiceLineBody),
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	plan := payload["plan"].(map[string]any)
	price := payload["price"].(map[string]any)
	lines := payload["lines"].([]any)
	if _, ok := plan["entitlements_json"].(map[string]any); !ok {
		t.Fatalf("entitlements_json was encoded as a string: %s", encoded)
	}
	if _, ok := price["tiers_json"].([]any); !ok {
		t.Fatalf("tiers_json was encoded as a string: %s", encoded)
	}
	if _, ok := lines[0].(map[string]any)["metadata_json"].(map[string]any); !ok {
		t.Fatalf("metadata_json was encoded as a string: %s", encoded)
	}
}

func TestBillingRequestAcceptsLegacyJSONString(t *testing.T) {
	t.Parallel()
	got := rawObject(json.RawMessage(`"{\"seats\":10}"`))
	if string(got) != `{"seats":10}` {
		t.Fatalf("rawObject() = %s", got)
	}
}
