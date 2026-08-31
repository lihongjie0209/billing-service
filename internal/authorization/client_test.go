package authorization

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestForwardCallerCredentialFromHTTPContext(t *testing.T) {
	t.Parallel()
	ctx := forwardCallerCredential(WithCallerCredential(context.Background(), "Bearer caller-token"))
	values := outgoingValues(ctx)
	if len(values) != 1 || values[0] != "Bearer caller-token" {
		t.Fatalf("authorization=%v", values)
	}
}
func TestForwardCallerCredentialDoesNotOverrideExplicitOutgoingValue(t *testing.T) {
	t.Parallel()
	ctx := metadata.AppendToOutgoingContext(WithCallerCredential(context.Background(), "Bearer caller-token"), "authorization", "PSK service-token")
	ctx = forwardCallerCredential(ctx)
	values := outgoingValues(ctx)
	if len(values) != 1 || values[0] != "PSK service-token" {
		t.Fatalf("authorization=%v", values)
	}
}

func outgoingValues(ctx context.Context) []string {
	values, _ := metadata.FromOutgoingContext(ctx)
	return values.Get("authorization")
}
