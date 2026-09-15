package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBillingTablesHaveDatabaseOwnedAuditShapeForEveryDialect(t *testing.T) {
	t.Parallel()
	tables := []string{"plans", "usage_prices", "subscriptions", "subscription_claims", "invoices", "invoice_lines", "invoice_generation_keys", "payment_attempts", "payment_provider_events", "refunds", "billing_outbox_events"}
	for _, dialect := range []string{"postgres", "kingbase", "mysql"} {
		dialect := dialect
		t.Run(dialect, func(t *testing.T) {
			t.Parallel()
			content, err := os.ReadFile(filepath.Join("..", "..", "migrations", dialect, "000006_database_audit.up.sql"))
			if err != nil {
				t.Fatal(err)
			}
			text := strings.ToLower(string(content))
			for _, table := range tables {
				if !strings.Contains(text, "alter table "+table) || !strings.Contains(text, "deleted_at") || !strings.Contains(text, "deleted_by") {
					t.Fatalf("%s migration lacks logical-delete audit shape for %s", dialect, table)
				}
			}
			if dialect == "mysql" {
				for _, suffix := range []string{"_audit_bi", "_audit_bu", "_audit_bd"} {
					if strings.Count(text, suffix) != 10 {
						t.Fatalf("%s migration has %d %s triggers, want 10", dialect, strings.Count(text, suffix), suffix)
					}
				}
			} else if strings.Count(text, "execute function billing_audit_row()") != 10 {
				t.Fatalf("%s migration lacks complete trigger coverage", dialect)
			}
			if !strings.Contains(text, "bounded-retention exception") {
				t.Fatal("outbox audit exception must be explicit")
			}
		})
	}
}

func TestBillingRepositoryExcludesLogicalDeletesAndDoesNotPhysicallyDeleteBusinessRows(t *testing.T) {
	t.Parallel()
	content, err := os.ReadFile(filepath.Join("..", "billing", "repository.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if strings.Contains(text, "DELETE FROM usage_prices") || strings.Contains(text, "DELETE FROM subscription_claims") {
		t.Fatal("business rows must be logically deleted")
	}
	for _, table := range []string{"plans", "usage_prices", "subscriptions", "invoices", "invoice_lines", "payment_attempts", "refunds"} {
		if !strings.Contains(text, table) || !strings.Contains(text, "deleted_at IS NULL") {
			t.Fatalf("repository lacks logical-delete filtering for %s", table)
		}
	}
}
