package billing

import (
	"strings"
	"testing"
)

func TestRefundInsertColumnAndValueCountsMatch(t *testing.T) {
	t.Parallel()
	columnCount := strings.Count(refundColumns, ",") + 1
	placeholderCount := strings.Count(refundInsertValues, "?")
	if columnCount != placeholderCount {
		t.Fatalf("refund insert has %d columns and %d placeholders", columnCount, placeholderCount)
	}
}
