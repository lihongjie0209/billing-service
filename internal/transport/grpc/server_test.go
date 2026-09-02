package grpctransport

import (
	"context"
	"testing"
	"time"

	"github.com/lihongjie0209/billing-service/internal/auth"
	"github.com/lihongjie0209/billing-service/internal/config"
	"github.com/lihongjie0209/billing-service/internal/requestid"
	platformauthz "github.com/lihongjie0209/microservice-platform-go/authz"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	billingv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/billing/v1"
	exportv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/export/v1"
	importv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/import/v1"
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

func TestBillingRequirementCoversEveryBusinessMethod(t *testing.T) {
	t.Parallel()
	methods := []string{
		billingv1.BillingService_CreatePlan_FullMethodName, billingv1.BillingService_UpdatePlan_FullMethodName, billingv1.BillingService_GetPlan_FullMethodName, billingv1.BillingService_ListPlans_FullMethodName, billingv1.BillingService_UpsertUsagePrice_FullMethodName, billingv1.BillingService_DeleteUsagePrice_FullMethodName,
		billingv1.BillingService_CreateSubscription_FullMethodName, billingv1.BillingService_ChangeSubscription_FullMethodName, billingv1.BillingService_CancelSubscription_FullMethodName, billingv1.BillingService_GetSubscription_FullMethodName, billingv1.BillingService_ListSubscriptions_FullMethodName,
		billingv1.BillingService_PreviewInvoice_FullMethodName, billingv1.BillingService_GenerateInvoice_FullMethodName, billingv1.BillingService_FinalizeInvoice_FullMethodName, billingv1.BillingService_VoidInvoice_FullMethodName, billingv1.BillingService_GetInvoice_FullMethodName, billingv1.BillingService_ListInvoices_FullMethodName,
		billingv1.BillingService_CreatePaymentAttempt_FullMethodName, billingv1.BillingService_ApplyPaymentResult_FullMethodName, billingv1.BillingService_RecordRefund_FullMethodName, billingv1.BillingService_ReconcilePayment_FullMethodName,
	}
	for _, method := range methods {
		requirement, ok := billingRequirement(method)
		if !ok || requirement.Resource == "" || requirement.Action == "" {
			t.Fatalf("method %q requirement = %+v, %v", method, requirement, ok)
		}
	}
}

func TestProviderMethodsUsePSKCapabilityWithoutRBAC(t *testing.T) {
	t.Parallel()
	const key = "01234567890123456789012345678901"
	authService := auth.New(config.Config{JWT: config.JWT{Issuer: "test", Secret: key, TTL: time.Hour}})
	methods := []string{
		exportv1.ExportProviderService_DescribeDataset_FullMethodName,
		exportv1.ExportProviderService_StreamRows_FullMethodName,
		importv1.ImportProviderService_DescribeImportDataset_FullMethodName,
		importv1.ImportProviderService_ValidateRows_FullMethodName,
		importv1.ImportProviderService_ApplyRows_FullMethodName,
	}
	cfg := config.Auth{PSK: config.PSK{Enabled: true, Key: key, GRPCMethods: []string{
		"/platform.export.v1.ExportProviderService/*",
		"/platform.import.v1.ImportProviderService/*",
	}}}
	for _, method := range methods {
		if requirement, ok := billingRequirement(method); ok {
			t.Fatalf("provider method %q has redundant RBAC requirement %+v", method, requirement)
		}
		ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "PSK "+key))
		if _, err := authenticateGRPC(ctx, method, authService, cfg); err != nil {
			t.Fatalf("provider method %q rejected capability PSK: %v", method, err)
		}
		if _, err := authenticateGRPC(t.Context(), method, authService, cfg); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("provider method %q accepted missing PSK: %v", method, err)
		}
	}
}

func TestBillingRequirementUsesResourceOwnershipScope(t *testing.T) {
	t.Parallel()
	for _, method := range []string{
		billingv1.BillingService_CreatePlan_FullMethodName,
		billingv1.BillingService_UpdatePlan_FullMethodName,
		billingv1.BillingService_GetPlan_FullMethodName,
		billingv1.BillingService_ListPlans_FullMethodName,
		billingv1.BillingService_UpsertUsagePrice_FullMethodName,
		billingv1.BillingService_DeleteUsagePrice_FullMethodName,
	} {
		requirement, ok := billingRequirement(method)
		if !ok || requirement.Scope != platformauthz.ScopePlatform {
			t.Fatalf("method %q scope = %q, want %q", method, requirement.Scope, platformauthz.ScopePlatform)
		}
	}
	for _, method := range []string{
		billingv1.BillingService_ListSubscriptions_FullMethodName,
		billingv1.BillingService_ListInvoices_FullMethodName,
	} {
		requirement, ok := billingRequirement(method)
		if !ok || requirement.Scope != platformauthz.ScopePrincipal {
			t.Fatalf("method %q scope = %q, want %q", method, requirement.Scope, platformauthz.ScopePrincipal)
		}
	}
}
