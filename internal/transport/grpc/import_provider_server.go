package grpctransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/lihongjie0209/billing-service/internal/billing"
	importv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/import/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const planImportDataset = "billing.plans"

var planImportColumns = []*importv1.ImportColumn{
	{Key: "code", Title: "Code", Type: "string", Required: true, Example: "pro-monthly"},
	{Key: "name", Title: "Name", Type: "string", Required: true, Example: "Pro"},
	{Key: "description", Title: "Description", Type: "string"},
	{Key: "currency", Title: "Currency", Type: "string", Required: true, Example: "CNY"},
	{Key: "billing_interval", Title: "Billing interval", Type: "string", Required: true, Example: "month"},
	{Key: "base_amount_minor", Title: "Base amount (minor units)", Type: "integer", Required: true, Example: "9900"},
	{Key: "trial_days", Title: "Trial days", Type: "integer", Example: "0"},
	{Key: "entitlements_json", Title: "Entitlements JSON", Type: "object", Example: `{}`},
}

type importProviderServer struct {
	importv1.UnimplementedImportProviderServiceServer
	service *billing.Service
}

func (s *importProviderServer) DescribeImportDataset(ctx context.Context, r *importv1.DescribeImportDatasetRequest) (*importv1.DescribeImportDatasetResponse, error) {
	if err := authorizeProviderScope(ctx, r.GetTenantId(), r.GetApplicationId()); err != nil {
		return nil, err
	}
	if r.GetDatasetCode() != planImportDataset {
		return nil, status.Error(codes.NotFound, "dataset not found")
	}
	return &importv1.DescribeImportDatasetResponse{Dataset: &importv1.ImportDatasetDescriptor{Code: planImportDataset, Title: "Billing plan drafts", Columns: planImportColumns, Formats: []string{"csv", "jsonl", "xlsx"}, MaxBatchSize: 100, SupportsDryRun: true}}, nil
}

func (s *importProviderServer) ValidateRows(stream importv1.ImportProviderService_ValidateRowsServer) error {
	var tenant, application, job string
	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := validateImportStreamRequest(stream.Context(), request.GetTenantId(), request.GetApplicationId(), request.GetDatasetCode(), request.GetJobId(), &tenant, &application, &job); err != nil {
			return err
		}
		normalized := make([]*structpb.Struct, 0, len(request.GetRows()))
		issues := make([]*importv1.RowIssue, 0)
		for index, row := range request.GetRows() {
			rowNumber := request.GetFirstRowNumber() + int64(index)
			plan, rowIssues := decodePlanRow(row.AsMap(), rowNumber)
			issues = append(issues, rowIssues...)
			if len(rowIssues) == 0 {
				value, encodeErr := structpb.NewStruct(planImportRow(plan))
				if encodeErr != nil {
					return status.Error(codes.Internal, "encode normalized plan")
				}
				normalized = append(normalized, value)
			}
		}
		if err := stream.Send(&importv1.ValidateRowsResponse{BatchNumber: request.GetBatchNumber(), NormalizedRows: normalized, Issues: issues}); err != nil {
			return err
		}
	}
}

func (s *importProviderServer) ApplyRows(stream importv1.ImportProviderService_ApplyRowsServer) error {
	var tenant, application, job string
	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := validateImportStreamRequest(stream.Context(), request.GetTenantId(), request.GetApplicationId(), request.GetDatasetCode(), request.GetJobId(), &tenant, &application, &job); err != nil {
			return err
		}
		if strings.TrimSpace(request.GetIdempotencyKey()) == "" {
			return status.Error(codes.InvalidArgument, "idempotency_key is required")
		}
		issues := make([]*importv1.RowIssue, 0)
		var applied int64
		for index, row := range request.GetRows() {
			rowNumber := int64(index + 1)
			plan, rowIssues := decodePlanRow(row.AsMap(), rowNumber)
			if len(rowIssues) > 0 {
				issues = append(issues, rowIssues...)
				continue
			}
			if _, _, err := s.service.ImportPlan(stream.Context(), plan); err != nil {
				issues = append(issues, &importv1.RowIssue{RowNumber: rowNumber, Code: "apply_conflict", Message: "plan could not be applied"})
				continue
			}
			applied++
		}
		if err := stream.Send(&importv1.ApplyRowsResponse{BatchNumber: request.GetBatchNumber(), AppliedRows: applied, Issues: issues}); err != nil {
			return err
		}
	}
}

func validateImportStreamRequest(ctx context.Context, tenantID, applicationID, dataset, jobID string, tenant, application, job *string) error {
	if err := authorizeProviderScope(ctx, tenantID, applicationID); err != nil {
		return err
	}
	if dataset != planImportDataset || strings.TrimSpace(jobID) == "" {
		return status.Error(codes.InvalidArgument, "valid dataset_code and job_id are required")
	}
	if *tenant == "" {
		*tenant, *application, *job = tenantID, applicationID, jobID
	}
	if tenantID != *tenant || applicationID != *application || jobID != *job {
		return status.Error(codes.InvalidArgument, "a stream cannot mix tenants, applications or jobs")
	}
	return nil
}

func decodePlanRow(row map[string]any, rowNumber int64) (billing.Plan, []*importv1.RowIssue) {
	issues := make([]*importv1.RowIssue, 0)
	required := func(key string) string {
		value := strings.TrimSpace(fmt.Sprint(row[key]))
		if value == "" || value == "<nil>" {
			issues = append(issues, &importv1.RowIssue{RowNumber: rowNumber, ColumnKey: key, Code: "required", Message: key + " is required"})
			return ""
		}
		return value
	}
	base, err := importInt64(row["base_amount_minor"], true)
	if err != nil || base < 0 {
		issues = append(issues, &importv1.RowIssue{RowNumber: rowNumber, ColumnKey: "base_amount_minor", Code: "invalid_integer", Message: "base_amount_minor must be a non-negative integer"})
	}
	trial, err := importInt64(row["trial_days"], false)
	if err != nil || trial < 0 || trial > math.MaxInt32 {
		issues = append(issues, &importv1.RowIssue{RowNumber: rowNumber, ColumnKey: "trial_days", Code: "invalid_integer", Message: "trial_days must be a non-negative 32-bit integer"})
	}
	entitlements := "{}"
	if value, ok := row["entitlements_json"]; ok && value != nil && fmt.Sprint(value) != "" {
		switch typed := value.(type) {
		case string:
			entitlements = typed
		default:
			encoded, encodeErr := json.Marshal(typed)
			if encodeErr != nil {
				issues = append(issues, &importv1.RowIssue{RowNumber: rowNumber, ColumnKey: "entitlements_json", Code: "invalid_json", Message: "entitlements_json must be valid JSON"})
			} else {
				entitlements = string(encoded)
			}
		}
	}
	if !json.Valid([]byte(entitlements)) {
		issues = append(issues, &importv1.RowIssue{RowNumber: rowNumber, ColumnKey: "entitlements_json", Code: "invalid_json", Message: "entitlements_json must be valid JSON"})
	}
	value := billing.Plan{Code: required("code"), Name: required("name"), Description: optionalImportString(row["description"]), Currency: required("currency"), BillingInterval: required("billing_interval"), BaseAmountMinor: base, TrialDays: int32(trial), EntitlementsJSON: entitlements}
	if len(issues) == 0 {
		normalized, err := billing.NormalizeImportedPlan(value)
		if err != nil {
			issues = append(issues, &importv1.RowIssue{RowNumber: rowNumber, Code: "invalid_definition", Message: "plan definition is invalid"})
		} else {
			value = normalized
		}
	}
	return value, issues
}

func optionalImportString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func importInt64(value any, required bool) (int64, error) {
	if value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
		if required {
			return 0, errors.New("value is required")
		}
		return 0, nil
	}
	switch typed := value.(type) {
	case float64:
		if typed != math.Trunc(typed) || typed < math.MinInt64 || typed > math.MaxInt64 {
			return 0, errors.New("not an integer")
		}
		return int64(typed), nil
	default:
		return strconv.ParseInt(strings.TrimSpace(fmt.Sprint(value)), 10, 64)
	}
}

func planImportRow(value billing.Plan) map[string]any {
	return map[string]any{"code": value.Code, "name": value.Name, "description": value.Description, "currency": value.Currency, "billing_interval": value.BillingInterval, "base_amount_minor": strconv.FormatInt(value.BaseAmountMinor, 10), "trial_days": strconv.FormatInt(int64(value.TrialDays), 10), "entitlements_json": value.EntitlementsJSON}
}
