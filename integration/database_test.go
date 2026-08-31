//go:build integration

package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lihongjie0209/billing-service/internal/billing"
	"github.com/lihongjie0209/billing-service/internal/config"
	appdb "github.com/lihongjie0209/billing-service/internal/database"
	"github.com/lihongjie0209/billing-service/internal/migration"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestRepositoryAndMigrations(t *testing.T) {
	for _, databaseType := range []string{"postgres", "mysql"} {
		t.Run(databaseType, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
			defer cancel()
			dsn, migrationURL := startDatabase(t, ctx, databaseType)
			migrationPath, err := filepath.Abs(filepath.Join("..", "migrations", databaseType))
			if err != nil {
				t.Fatal(err)
			}
			schema := ""
			if databaseType == "postgres" {
				schema = "integration_postgres"
			}
			migrationCfg := config.Migration{Path: migrationPath, DatabaseURL: migrationURL, Table: "integration_" + databaseType + "_schema_migrations", Schema: schema, CreateSchema: schema != ""}
			migrationErrors := make(chan error, 3)
			var migrations sync.WaitGroup
			for range 3 {
				migrations.Add(1)
				go func() {
					defer migrations.Done()
					migrationErrors <- migration.Run(migrationCfg, "up", 0)
				}()
			}
			migrations.Wait()
			close(migrationErrors)
			for err := range migrationErrors {
				if err != nil {
					t.Fatalf("concurrent migration up: %v", err)
				}
			}

			db, err := appdb.Open(ctx, config.Database{Type: databaseType, DSN: dsn, Schema: schema, MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Minute, PingTimeout: 10 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			var billingTables int
			if databaseType == "postgres" {
				if err := db.GetContext(ctx, &billingTables, `SELECT count(*) FROM pg_tables WHERE schemaname = current_schema() AND tablename IN ('plans','subscriptions','invoices','payment_attempts','refunds','billing_outbox_events')`); err != nil {
					t.Fatal(err)
				}
				var timezone string
				if err := db.GetContext(ctx, &timezone, `SHOW TIMEZONE`); err != nil || timezone != "Asia/Shanghai" {
					t.Fatalf("timezone=%q err=%v", timezone, err)
				}
			} else if err := db.GetContext(ctx, &billingTables, `SELECT count(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('plans','subscriptions','invoices','payment_attempts','refunds','billing_outbox_events')`); err != nil {
				t.Fatal(err)
			}
			if billingTables != 6 {
				t.Fatalf("billing table count = %d, want 6", billingTables)
			}
			repository := billing.NewRepository(db)
			service := billing.NewService(repository, appdb.NewTransactor(db), zeroUsage{})
			actorCtx := platformprincipal.WithContext(ctx, platformprincipal.Principal{ID: "integration-service", Type: platformprincipal.TypeServiceAccount})
			plan, err := service.CreatePlan(actorCtx, billing.Plan{Code: "integration." + databaseType, Name: "Integration", Currency: "CNY", BillingInterval: "month", BaseAmountMinor: 100})
			if err != nil {
				t.Fatalf("create plan: %v", err)
			}
			plan.Name, plan.Status = "Integration Active", "active"
			plan, err = service.UpdatePlan(actorCtx, plan, plan.Version)
			if err != nil {
				t.Fatalf("activate plan: %v", err)
			}
			subscription, err := service.CreateSubscription(actorCtx, "tenant-integration", plan.ID, time.Now(), "")
			if err != nil {
				t.Fatalf("create subscription: %v", err)
			}
			invoice, duplicate, err := service.GenerateInvoice(actorCtx, "tenant-integration", subscription.ID, time.Time{}, time.Time{}, "invoice-integration-key")
			if err != nil || duplicate {
				t.Fatalf("generate invoice duplicate=%v err=%v", duplicate, err)
			}
			replayed, replayedDuplicate, err := service.GenerateInvoice(actorCtx, "tenant-integration", subscription.ID, time.Time{}, time.Time{}, "invoice-integration-key")
			if err != nil || !replayedDuplicate || replayed.Invoice.ID != invoice.Invoice.ID {
				t.Fatalf("replay invoice=%s duplicate=%v err=%v", replayed.Invoice.ID, replayedDuplicate, err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if err := migration.Run(migrationCfg, "down", 0); err != nil {
				t.Fatalf("migration down: %v", err)
			}
		})
	}
}

type zeroUsage struct{}

func (zeroUsage) Total(context.Context, string, string, time.Time, time.Time) (int64, error) {
	return 0, nil
}

func startDatabase(t *testing.T, ctx context.Context, databaseType string) (string, string) {
	t.Helper()
	switch databaseType {
	case "postgres":
		container, err := postgres.Run(ctx, "postgres:17-alpine", postgres.WithDatabase("app"), postgres.WithUsername("app"), postgres.WithPassword("app"), postgres.BasicWaitStrategies(), postgres.WithSQLDriver("pgx"))
		if err != nil {
			t.Fatal(err)
		}
		testcontainers.CleanupContainer(t, container)
		dsn, err := container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatal(err)
		}
		return dsn, dsn
	case "mysql":
		container, err := mysql.Run(ctx, "mysql:8.4", mysql.WithDatabase("app"), mysql.WithUsername("app"), mysql.WithPassword("app"))
		if err != nil {
			t.Fatal(err)
		}
		testcontainers.CleanupContainer(t, container)
		dsn, err := container.ConnectionString(ctx, "parseTime=true")
		if err != nil {
			t.Fatal(err)
		}
		migrationDSN, err := container.ConnectionString(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return dsn, "mysql://" + migrationDSN
	default:
		t.Fatal(fmt.Errorf("unsupported database %q", databaseType))
		return "", ""
	}
}
