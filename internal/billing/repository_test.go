package billing

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestSQLRepository_CreateSubscriptionWritesAbsentPendingPlanAsNull(t *testing.T) {
	t.Parallel()

	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	db := sqlx.NewDb(database, "sqlmock")
	repository := &SQLRepository{db: db}
	now := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	subscription := Subscription{
		ID:                 "subscription-1",
		TenantID:           "tenant-1",
		PlanID:             "plan-1",
		Status:             "active",
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		ExternalReference:  "external-1",
		PendingPlanID:      " ",
		Audit: Audit{
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
			CreatedBy: "user-1",
			UpdatedBy: "user-1",
		},
	}

	arguments := []driver.Value{
		subscription.ID, subscription.TenantID, subscription.PlanID, subscription.Status,
		subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd, false, nil,
		subscription.ExternalReference, nil, nil, int64(1), now, now, "user-1", "user-1",
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO subscriptions (" + subscriptionColumns + ") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")).
		WithArgs(arguments...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	if err := repository.CreateSubscription(context.Background(), db, subscription); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("verify SQL expectations: %v", err)
	}
}
