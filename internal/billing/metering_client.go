package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/lihongjie0209/billing-service/internal/outbound"
	commonv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/common/v1"
	meteringv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/metering/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MeteringClient struct {
	client meteringv1.MeteringServiceClient
}

func NewMeteringClient(registry *outbound.Registry) (UsageReader, error) {
	connection, ok := registry.GRPC("metering")
	if !ok {
		return unavailableUsageReader{}, nil
	}
	return &MeteringClient{client: meteringv1.NewMeteringServiceClient(connection)}, nil
}

type unavailableUsageReader struct{}

func (unavailableUsageReader) Total(context.Context, string, string, time.Time, time.Time) (int64, error) {
	return 0, fmt.Errorf("metering gRPC upstream is not configured")
}

func (c *MeteringClient) Total(ctx context.Context, tenantID, meterCode string, start, end time.Time) (int64, error) {
	response, err := c.client.QueryUsage(ctx, &meteringv1.QueryUsageRequest{TenantId: tenantID, MeterCode: meterCode, StartAt: timestamppb.New(start), EndAt: timestamppb.New(end), Granularity: "total", Page: &commonv1.PageRequest{Page: 1, PageSize: 1}})
	if err != nil {
		return 0, err
	}
	return response.GetTotalQuantity(), nil
}
