package grpctransport

import (
	"context"
	"testing"
	"time"

	"github.com/lihongjie0209/billing-service/internal/billing"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSelectInvoiceColumnsPreservesRequestedOrder(t *testing.T) {
	values, err := selectInvoiceColumns([]string{"total_minor", "id", "total_minor"})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0].GetKey() != "total_minor" || values[1].GetKey() != "id" {
		t.Fatalf("columns=%+v", values)
	}
	if _, err := selectInvoiceColumns([]string{"secret"}); err == nil {
		t.Fatal("unknown column accepted")
	}
}
func TestInvoiceRowPreservesInt64Precision(t *testing.T) {
	value := billing.Invoice{ID: "invoice-1", TotalMinor: 9_007_199_254_740_993, PeriodStart: time.Unix(0, 0), PeriodEnd: time.Unix(1, 0), Audit: billing.Audit{Version: 7, CreatedAt: time.Unix(2, 0), UpdatedAt: time.Unix(3, 0)}}
	columns, _ := selectInvoiceColumns([]string{"total_minor", "version"})
	row := invoiceRow(value, columns)
	if row["total_minor"] != "9007199254740993" || row["version"] != "7" {
		t.Fatalf("row=%+v", row)
	}
}
func TestAuthorizeExportTenant(t *testing.T) {
	user := platformprincipal.WithContext(context.Background(), platformprincipal.Principal{ID: "u1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	if err := authorizeExportTenant(user, "tenant-1"); err != nil {
		t.Fatal(err)
	}
	if err := authorizeExportTenant(user, "tenant-2"); err == nil {
		t.Fatal("cross tenant caller accepted")
	}
	service := platformprincipal.WithContext(context.Background(), platformprincipal.Principal{ID: "export", Type: platformprincipal.TypeServiceAccount})
	if err := authorizeExportTenant(service, "tenant-2"); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorizeProviderScopeRequiresTenantAndApplication(t *testing.T) {
	t.Parallel()
	service := platformprincipal.WithContext(context.Background(), platformprincipal.Principal{ID: "export", Type: platformprincipal.TypeServiceAccount})
	if err := authorizeProviderScope(service, "tenant-1", "application-1"); err != nil {
		t.Fatal(err)
	}
	for _, scope := range [][2]string{{"", "application-1"}, {"tenant-1", ""}} {
		if err := authorizeProviderScope(service, scope[0], scope[1]); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("scope %q/%q status = %s, err = %v", scope[0], scope[1], status.Code(err), err)
		}
	}
}

func TestParseInvoiceExportQueryUsesProtocolApplicationScope(t *testing.T) {
	t.Parallel()
	query, err := parseInvoiceExportQuery("application-1", `{ "status": "open" }`)
	if err != nil || query.Status != "open" {
		t.Fatalf("query = %+v, err = %v", query, err)
	}
	query, err = parseInvoiceExportQuery("application-1", `{"application_id":"application-1","status":"paid"}`)
	if err != nil || query.Status != "paid" {
		t.Fatalf("legacy query = %+v, err = %v", query, err)
	}
}

func TestParseInvoiceExportQueryRejectsMissingOrConflictingScope(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name          string
		applicationID string
		raw           string
	}{
		{name: "missing top-level scope", raw: `{}`},
		{name: "conflicting legacy scope", applicationID: "application-1", raw: `{"application_id":"application-2"}`},
		{name: "non-object query", applicationID: "application-1", raw: `null`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseInvoiceExportQuery(test.applicationID, test.raw)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("status = %s, err = %v", status.Code(err), err)
			}
		})
	}
}
