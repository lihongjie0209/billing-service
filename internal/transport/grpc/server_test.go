package grpctransport

import (
	"context"
	"testing"
	"time"

	"github.com/lihongjie0209/billing-service/internal/auth"
	"github.com/lihongjie0209/billing-service/internal/config"
	"github.com/lihongjie0209/billing-service/internal/requestid"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRequestIDInterceptorPropagatesHeaderAndContext(t *testing.T) {
	t.Parallel()
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-request-id", "grpc-test-1"))
	_, err := requestIDInterceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/platform.billing.v1.BillingService/GetPlan"}, func(ctx context.Context, _ any) (any, error) {
		if id, ok := requestid.FromContext(ctx); !ok || id != "grpc-test-1" {
			t.Fatalf("request id = %q, %v", id, ok)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAuthenticateGRPC_PSKWildcard(t *testing.T) {
	t.Parallel()
	const key = "01234567890123456789012345678901"
	authService := auth.New(config.Config{JWT: config.JWT{Issuer: "test", Secret: key, TTL: time.Hour}})
	cfg := config.Auth{
		SkipGRPCMethods: []string{"/platform.billing.v1.BillingService/*"},
		PSK:             config.PSK{Enabled: true, Key: key, GRPCMethods: []string{"/platform.billing.v1.BillingService/*"}},
	}
	for _, test := range []struct {
		name   string
		header string
		code   codes.Code
	}{
		{name: "valid", header: "PSK " + key, code: codes.OK},
		{name: "PSK precedes skip", code: codes.Unauthenticated},
		{name: "bearer rejected", header: "Bearer " + key, code: codes.Unauthenticated},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", test.header))
			authenticated, err := authenticateGRPC(ctx, "/platform.billing.v1.BillingService/GetPlan", authService, cfg)
			if got := status.Code(err); got != test.code {
				t.Fatalf("status code = %s, want %s", got, test.code)
			}
			if test.code == codes.OK {
				value, ok := platformprincipal.FromContext(authenticated)
				if !ok || value.ID != "billing-service:psk" || value.Type != platformprincipal.TypeServiceAccount {
					t.Fatalf("principal = %#v, %v", value, ok)
				}
			}
		})
	}
}

func TestAuthenticateGRPC_JWTInjectsPrincipal(t *testing.T) {
	t.Parallel()
	const key = "01234567890123456789012345678901"
	service := auth.New(config.Config{JWT: config.JWT{Issuer: "test", Secret: key, TTL: time.Hour}, Auth: config.Auth{ClientID: "client", ClientSecret: "secret"}})
	token, err := service.Issue("user-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+token))
	ctx, err = authenticateGRPC(ctx, "/platform.billing.v1.BillingService/GetPlan", service, config.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	value, ok := platformprincipal.FromContext(ctx)
	if !ok || value.ID != "user-1" || value.Type != platformprincipal.TypeUser {
		t.Fatalf("principal = %#v, %v", value, ok)
	}
}

func TestBillingRequirementOnlyProtectsPlanMutations(t *testing.T) {
	t.Parallel()
	requirement, ok := billingRequirement("/platform.billing.v1.BillingService/CreatePlan")
	if !ok || requirement.Resource != "billing.plan" || requirement.Action != "create" {
		t.Fatalf("requirement=%+v protected=%v", requirement, ok)
	}
	if _, ok := billingRequirement("/platform.billing.v1.BillingService/GetInvoice"); ok {
		t.Fatal("tenant-scoped reads are enforced in the domain layer")
	}
}
