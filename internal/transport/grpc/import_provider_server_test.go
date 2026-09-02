package grpctransport

import (
	"context"
	"testing"

	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

func TestValidateImportStreamRequestPinsApplicationScope(t *testing.T) {
	t.Parallel()
	ctx := platformprincipal.WithContext(context.Background(), platformprincipal.Principal{ID: "import-service", Type: platformprincipal.TypeServiceAccount})
	var tenant, application, job string
	if err := validateImportStreamRequest(ctx, "tenant-1", "application-1", planImportDataset, "job-1", &tenant, &application, &job); err != nil {
		t.Fatal(err)
	}
	if tenant != "tenant-1" || application != "application-1" || job != "job-1" {
		t.Fatalf("scope = %q/%q/%q", tenant, application, job)
	}
	if err := validateImportStreamRequest(ctx, "tenant-1", "application-2", planImportDataset, "job-1", &tenant, &application, &job); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("cross-application status = %s, err = %v", status.Code(err), err)
	}
}

func TestValidateImportStreamRequestRequiresApplicationScope(t *testing.T) {
	t.Parallel()
	ctx := platformprincipal.WithContext(context.Background(), platformprincipal.Principal{ID: "import-service", Type: platformprincipal.TypeServiceAccount})
	var tenant, application, job string
	err := validateImportStreamRequest(ctx, "tenant-1", "", planImportDataset, "job-1", &tenant, &application, &job)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, err = %v", status.Code(err), err)
	}
}
