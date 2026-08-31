package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/lihongjie0209/billing-service/internal/config"
	"github.com/lihongjie0209/billing-service/internal/grpcclient"
	"github.com/lihongjie0209/microservice-platform-go/exportprovider"
	"github.com/lihongjie0209/microservice-platform-go/serviceregistry"
	registryv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/registry/v1"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

type registryRuntime struct {
	cfg        config.Config
	logger     *slog.Logger
	connection *grpc.ClientConn
	registrant *serviceregistry.Registrant
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func newRegistryRuntime(lifecycle fx.Lifecycle, cfg config.Config, logger *slog.Logger) (*registryRuntime, error) {
	runtime := &registryRuntime{cfg: cfg, logger: logger}
	if !cfg.ServiceRegistry.Enabled {
		lifecycle.Append(fx.Hook{OnStart: runtime.start, OnStop: runtime.stop})
		return runtime, nil
	}
	metadata, err := exportprovider.Metadata([]exportprovider.Dataset{{Code: "billing.invoices", Title: "Billing invoices", Formats: []string{"csv", "jsonl", "xlsx"}, SupportsSnapshot: false}})
	if err != nil {
		return nil, err
	}
	connection, err := grpcclient.Dial(grpcclient.Config{Name: "service-registry-service", Target: cfg.ServiceRegistry.Target, Timeout: 3 * time.Second, PSK: cfg.ServiceRegistry.PSK, TLS: grpcclient.TLSConfig{Enabled: cfg.ServiceRegistry.TLS.Enabled, ServerName: cfg.ServiceRegistry.TLS.ServerName, CAFile: cfg.ServiceRegistry.TLS.CAFile, CertFile: cfg.ServiceRegistry.TLS.CertFile, KeyFile: cfg.ServiceRegistry.TLS.KeyFile, AllowInsecureToken: cfg.ServiceRegistry.AllowInsecure}})
	if err != nil {
		return nil, err
	}
	runtime.connection = connection
	runtime.registrant, err = serviceregistry.NewRegistrant(registryv1.NewRegistryServiceClient(connection), serviceregistry.RegistrantConfig{Instance: &registryv1.ServiceInstance{ServiceName: cfg.App.Name, InstanceId: cfg.ServiceRegistry.InstanceID, Endpoint: cfg.ServiceRegistry.Endpoint, Metadata: metadata, Status: registryv1.InstanceStatus_INSTANCE_STATUS_HEALTHY, Weight: 1}, Lease: cfg.ServiceRegistry.Lease, HeartbeatInterval: cfg.ServiceRegistry.HeartbeatInterval})
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	lifecycle.Append(fx.Hook{OnStart: runtime.start, OnStop: runtime.stop})
	return runtime, nil
}
func (r *registryRuntime) start(context.Context) error {
	if r.registrant == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Go(func() {
		if err := r.registrant.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.logger.ErrorContext(ctx, "service registration stopped", "error", err)
		}
	})
	return nil
}
func (r *registryRuntime) stop(context.Context) error {
	if r.cancel != nil {
		r.cancel()
		r.wg.Wait()
	}
	if r.connection != nil {
		return r.connection.Close()
	}
	return nil
}

var ServiceRegistryModule = fx.Module("service-registry", fx.Provide(newRegistryRuntime), fx.Invoke(func(*registryRuntime) {}))
