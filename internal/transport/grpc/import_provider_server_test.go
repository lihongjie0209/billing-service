package grpctransport

import "testing"

func TestDecodePlanRowNormalizesPortableValues(t *testing.T) {
	t.Parallel()
	value, issues := decodePlanRow(map[string]any{"code": "PRO-MONTHLY", "name": "Pro", "currency": "cny", "billing_interval": "MONTH", "base_amount_minor": "9007199254740993", "trial_days": float64(7), "entitlements_json": map[string]any{"seats": float64(10)}}, 4)
	if len(issues) != 0 || value.BaseAmountMinor != 9_007_199_254_740_993 || value.TrialDays != 7 {
		t.Fatalf("value=%+v issues=%+v", value, issues)
	}
	if value.Code != "pro-monthly" || value.Currency != "CNY" || value.BillingInterval != "month" || value.Description != "" {
		t.Fatalf("normalized value=%+v", value)
	}
	row := planImportRow(value)
	if row["base_amount_minor"] != "9007199254740993" || row["trial_days"] != "7" {
		t.Fatalf("row=%+v", row)
	}
}

func TestDecodePlanRowReturnsColumnIssues(t *testing.T) {
	t.Parallel()
	_, issues := decodePlanRow(map[string]any{"base_amount_minor": "invalid", "trial_days": "-1", "entitlements_json": "{"}, 9)
	if len(issues) < 6 {
		t.Fatalf("issues=%+v", issues)
	}
	for _, issue := range issues {
		if issue.GetRowNumber() != 9 || issue.GetCode() == "" {
			t.Fatalf("issue=%+v", issue)
		}
	}
}
