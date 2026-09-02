package grpctransport

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/lihongjie0209/billing-service/internal/billing"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	exportv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/export/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const invoiceDataset = "billing.invoices"

var invoiceColumns = []*exportv1.ExportColumn{{Key: "id", Title: "ID", Type: "string"}, {Key: "number", Title: "Invoice number", Type: "string"}, {Key: "subscription_id", Title: "Subscription ID", Type: "string"}, {Key: "currency", Title: "Currency", Type: "string"}, {Key: "status", Title: "Status", Type: "string"}, {Key: "period_start", Title: "Period start", Type: "datetime", Format: time.RFC3339}, {Key: "period_end", Title: "Period end", Type: "datetime", Format: time.RFC3339}, {Key: "subtotal_minor", Title: "Subtotal minor", Type: "integer"}, {Key: "discount_minor", Title: "Discount minor", Type: "integer"}, {Key: "tax_minor", Title: "Tax minor", Type: "integer"}, {Key: "total_minor", Title: "Total minor", Type: "integer"}, {Key: "paid_minor", Title: "Paid minor", Type: "integer"}, {Key: "refunded_minor", Title: "Refunded minor", Type: "integer"}, {Key: "due_at", Title: "Due at", Type: "datetime", Format: time.RFC3339}, {Key: "created_at", Title: "Created at", Type: "datetime", Format: time.RFC3339}, {Key: "updated_at", Title: "Updated at", Type: "datetime", Format: time.RFC3339}, {Key: "version", Title: "Version", Type: "integer"}}

var invoiceQueryFields = []*exportv1.ExportQueryField{
	{Key: "status", Title: "Status", Type: "string", Options: []string{"draft", "open", "paid", "void"}, Description: "Exact invoice status"},
	{Key: "created_from", Title: "Created from", Type: "datetime", Format: time.RFC3339},
	{Key: "created_to", Title: "Created to", Type: "datetime", Format: time.RFC3339},
}

type exportProviderServer struct {
	exportv1.UnimplementedExportProviderServiceServer
	service *billing.Service
}

type invoiceExportQuery struct {
	ApplicationID string `json:"application_id"`
	Status        string `json:"status"`
	CreatedFrom   string `json:"created_from"`
	CreatedTo     string `json:"created_to"`
}

func (s *exportProviderServer) DescribeDataset(ctx context.Context, r *exportv1.DescribeDatasetRequest) (*exportv1.DescribeDatasetResponse, error) {
	if err := authorizeProviderScope(ctx, r.GetTenantId(), r.GetApplicationId()); err != nil {
		return nil, err
	}
	if r.GetDatasetCode() != invoiceDataset {
		return nil, status.Error(codes.NotFound, "dataset not found")
	}
	return &exportv1.DescribeDatasetResponse{Dataset: &exportv1.DatasetDescriptor{Code: invoiceDataset, Title: "Billing invoices", Columns: invoiceColumns, QueryFields: invoiceQueryFields, Formats: []string{"csv", "jsonl", "xlsx"}, SupportsSnapshot: false}}, nil
}
func (s *exportProviderServer) StreamRows(r *exportv1.StreamRowsRequest, stream exportv1.ExportProviderService_StreamRowsServer) error {
	if err := authorizeProviderScope(stream.Context(), r.GetTenantId(), r.GetApplicationId()); err != nil {
		return err
	}
	if r.GetDatasetCode() != invoiceDataset {
		return status.Error(codes.NotFound, "dataset not found")
	}
	columns, err := selectInvoiceColumns(r.GetSelectedColumns())
	if err != nil {
		return err
	}
	query, err := parseInvoiceExportQuery(r.GetApplicationId(), r.GetQueryJson())
	if err != nil {
		return err
	}
	from, err := parseOptionalTime(query.CreatedFrom)
	if err != nil {
		return status.Error(codes.InvalidArgument, "created_from must be RFC3339")
	}
	to, err := parseOptionalTime(query.CreatedTo)
	if err != nil {
		return status.Error(codes.InvalidArgument, "created_to must be RFC3339")
	}
	page := 1
	if r.GetCursor() != "" {
		page, err = strconv.Atoi(r.GetCursor())
		if err != nil || page < 1 {
			return status.Error(codes.InvalidArgument, "cursor is invalid")
		}
	}
	size := int(r.GetBatchSize())
	if size < 1 {
		size = 100
	}
	if size > 100 {
		size = 100
	}
	for {
		values, err := s.service.ListInvoices(stream.Context(), r.GetTenantId(), r.GetApplicationId(), query.Status, from, to, page, size)
		if err != nil {
			return billingError(err)
		}
		rows := make([]*structpb.Struct, len(values.Items))
		for i, value := range values.Items {
			row, err := structpb.NewStruct(invoiceRow(value, columns))
			if err != nil {
				return status.Error(codes.Internal, "encode invoice row")
			}
			rows[i] = row
		}
		done := len(values.Items) < size || int64(page*size) >= values.Total
		next := ""
		if !done {
			next = strconv.Itoa(page + 1)
		}
		response := &exportv1.StreamRowsResponse{Rows: rows, NextCursor: next, EstimatedTotalRows: values.Total, Done: done}
		if page == 1 {
			response.Columns = columns
		}
		if err := stream.Send(response); err != nil {
			return err
		}
		if done {
			return nil
		}
		page++
	}
}

func parseInvoiceExportQuery(applicationID, raw string) (invoiceExportQuery, error) {
	if applicationID == "" {
		return invoiceExportQuery{}, status.Error(codes.InvalidArgument, "application_id is required")
	}
	query := invoiceExportQuery{}
	if raw != "" {
		properties := map[string]json.RawMessage{}
		if err := json.Unmarshal([]byte(raw), &properties); err != nil || strings.TrimSpace(raw) == "null" {
			return invoiceExportQuery{}, status.Error(codes.InvalidArgument, "query_json must be an object")
		}
		for key := range properties {
			if key != "application_id" && key != "status" && key != "created_from" && key != "created_to" {
				return invoiceExportQuery{}, status.Errorf(codes.InvalidArgument, "query_json field %q is not supported", key)
			}
		}
		if err := json.Unmarshal([]byte(raw), &query); err != nil {
			return invoiceExportQuery{}, status.Error(codes.InvalidArgument, "query_json contains an invalid value")
		}
	}
	if query.ApplicationID != "" && query.ApplicationID != applicationID {
		return invoiceExportQuery{}, status.Error(codes.InvalidArgument, "query_json.application_id must match application_id")
	}
	if query.Status != "" && !map[string]bool{"draft": true, "open": true, "paid": true, "void": true}[query.Status] {
		return invoiceExportQuery{}, status.Error(codes.InvalidArgument, "query_json.status is not supported")
	}
	return query, nil
}
func selectInvoiceColumns(keys []string) ([]*exportv1.ExportColumn, error) {
	if len(keys) == 0 {
		return invoiceColumns, nil
	}
	byKey := map[string]*exportv1.ExportColumn{}
	for _, column := range invoiceColumns {
		byKey[column.GetKey()] = column
	}
	result := make([]*exportv1.ExportColumn, 0, len(keys))
	seen := map[string]bool{}
	for _, key := range keys {
		column, ok := byKey[key]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "unknown selected column %q", key)
		}
		if !seen[key] {
			result = append(result, column)
			seen[key] = true
		}
	}
	return result, nil
}
func invoiceRow(v billing.Invoice, columns []*exportv1.ExportColumn) map[string]any {
	all := map[string]any{"id": v.ID, "number": v.Number, "subscription_id": v.SubscriptionID, "currency": v.Currency, "status": v.Status, "period_start": v.PeriodStart.Format(time.RFC3339Nano), "period_end": v.PeriodEnd.Format(time.RFC3339Nano), "subtotal_minor": strconv.FormatInt(v.SubtotalMinor, 10), "discount_minor": strconv.FormatInt(v.DiscountMinor, 10), "tax_minor": strconv.FormatInt(v.TaxMinor, 10), "total_minor": strconv.FormatInt(v.TotalMinor, 10), "paid_minor": strconv.FormatInt(v.PaidMinor, 10), "refunded_minor": strconv.FormatInt(v.RefundedMinor, 10), "due_at": optionalTime(v.DueAt), "created_at": v.CreatedAt.Format(time.RFC3339Nano), "updated_at": v.UpdatedAt.Format(time.RFC3339Nano), "version": strconv.FormatInt(v.Version, 10)}
	result := make(map[string]any, len(columns))
	for _, column := range columns {
		result[column.GetKey()] = all[column.GetKey()]
	}
	return result
}
func parseOptionalTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}
func optionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
func authorizeExportTenant(ctx context.Context, tenantID string) error {
	principal, ok := platformprincipal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "authenticated caller is required")
	}
	if principal.Type != platformprincipal.TypeServiceAccount && principal.Type != platformprincipal.TypeSystem && principal.TenantID != tenantID {
		return status.Error(codes.PermissionDenied, "tenant access denied")
	}
	return nil
}

func authorizeProviderScope(ctx context.Context, tenantID, applicationID string) error {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(applicationID) == "" {
		return status.Error(codes.InvalidArgument, "tenant_id and application_id are required")
	}
	return authorizeExportTenant(ctx, tenantID)
}
