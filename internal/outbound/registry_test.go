package outbound

import (
	"testing"

	"github.com/lihongjie0209/billing-service/internal/config"
)

func TestGRPCClientConfigCarriesExplicitInsecureCredentialOptIn(t *testing.T) {
	t.Parallel()
	value := newGRPCClientConfig("application", config.GRPCUpstream{
		Target: "application-service:9090",
		Auth:   config.ClientAuth{Type: "psk", Token: "development-psk"},
		TLS:    config.ClientTLS{AllowInsecure: true},
	}, nil)
	if value.PSK != "development-psk" || !value.TLS.AllowInsecureToken {
		t.Fatalf("client config = %+v", value)
	}
}
