package rexportv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	exportv1 "the-dev-tools/spec/dist/buf/go/api/export/v1"
	"the-dev-tools/spec/dist/buf/go/api/export/v1/exportv1connect"

	"connectrpc.com/connect"
)

// ExportFormat represents the supported export formats
type ExportFormat string

const (
	ExportFormat_YAML ExportFormat = "YAML"
	ExportFormat_CURL ExportFormat = "CURL"
)

// ExportRequest represents a request to export data
type ExportRequest struct {
	WorkspaceID idwrap.IDWrap
	FileIDs     []idwrap.IDWrap
	Format      ExportFormat
	Simplified  bool
}

// ExportCurlRequest represents a request to export cURL commands
type ExportCurlRequest struct {
	WorkspaceID idwrap.IDWrap
	HTTPIDs     []idwrap.IDWrap
}

// ExportResponse represents the response from an export operation
type ExportResponse struct {
	Name string
	Data []byte
}

// ExportCurlResponse represents the response from a cURL export operation
type ExportCurlResponse struct {
	Data string
}

// ExportFilter represents filters for export operations
type ExportFilter struct {
	FileIDs    []idwrap.IDWrap
	HTTPIDs    []idwrap.IDWrap
	Format     ExportFormat
	Simplified bool
}

// WorkspaceExportData represents data exported from a workspace
type WorkspaceExportData struct {
	Workspace    *WorkspaceInfo
	Flows        []*FlowData
	HTTPRequests []*HTTPData
	Files        []*FileData
}

// WorkspaceInfo represents basic workspace information
type WorkspaceInfo struct {
	ID   idwrap.IDWrap
	Name string
}

// FlowData represents flow data for export
type FlowData struct {
	ID          idwrap.IDWrap
	Name        string
	Description string
	Variables   map[string]interface{}
	Steps       []interface{}
}

// HTTPData represents HTTP request/response data for export
type HTTPData struct {
	ID          idwrap.IDWrap
	Name        string
	Method      string
	Url         string
	Headers     map[string][]string
	Body        string
	QueryParams map[string][]string
}

// FileData represents file data for export
type FileData struct {
	ID   idwrap.IDWrap
	Name string
	Path string
	Data []byte
}

// Error definitions
var (
	ErrWorkspaceNotFound = fmt.Errorf("workspace not found")
	ErrPermissionDenied  = fmt.Errorf("permission denied")
	ErrExportFailed      = fmt.Errorf("export failed")
	ErrNoDataFound       = fmt.Errorf("no data found")
	ErrUnsupportedFormat = fmt.Errorf("unsupported format")
	ErrTimeout           = fmt.Errorf("operation timed out")
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewValidationErrorWithCause creates a new validation error with a cause
func NewValidationErrorWithCause(field string, cause error) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: cause.Error(),
	}
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	var valErr *ValidationError
	return errors.As(err, &valErr)
}

// ExportV2RPC implements the Connect RPC interface for export v2
type ExportV2RPC struct {
	db      *sql.DB
	service *Service
	logger  *slog.Logger
	ws      sworkspace.WorkspaceService
	us      suser.UserService
}

// NewExportV2RPC creates a new ExportV2RPC handler with modern services
func NewExportV2RPC(
	db *sql.DB,
	queries *gen.Queries,
	ws sworkspace.WorkspaceService,
	us suser.UserService,
	httpService *shttp.HTTPService,
	flowService *sflow.FlowService,
	fileService *sfile.FileService,
	logger *slog.Logger,
) *ExportV2RPC {
	// Create IOWorkspaceService
	ioWorkspaceService := ioworkspace.New(queries, logger)

	// Create simple storage with modern services
	storage := NewStorage(&ws, httpService, flowService, fileService)

	// Create simple exporter with IOWorkspaceService
	exporter := NewExporter(httpService, flowService, fileService, ioWorkspaceService)

	// Create simple validator
	validator := NewValidator(&us)

	// Create the main service
	service := NewService(exporter, validator, storage)

	return &ExportV2RPC{
		db:      db,
		service: service,
		logger:  logger,
		ws:      ws,
		us:      us,
	}
}

// CreateExportV2Service creates the service registration for rexportv2
func CreateExportV2Service(srv ExportV2RPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := exportv1connect.NewExportServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// Export implements the Export RPC method
func (h *ExportV2RPC) Export(ctx context.Context, req *connect.Request[exportv1.ExportRequest]) (*connect.Response[exportv1.ExportResponse], error) {
	h.logger.Info("Received Export request",
		"workspace_id", req.Msg.WorkspaceId,
		"file_ids_count", len(req.Msg.FileIds))

	// Convert protobuf request to internal request model
	exportReq, err := convertToExportRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Call the service to process the export
	response, err := h.service.Export(ctx, exportReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Convert internal response to protobuf response
	protoResp, err := convertToExportResponse(response)
	if err != nil {
		h.logger.Error("Response conversion failed",
			"workspace_id", req.Msg.WorkspaceId,
			"error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	h.logger.Info("Export completed successfully",
		"workspace_id", req.Msg.WorkspaceId,
		"export_name", protoResp.Name,
		"data_size", len(protoResp.Data))

	return connect.NewResponse(protoResp), nil
}

// ExportCurl implements the ExportCurl RPC method
func (h *ExportV2RPC) ExportCurl(ctx context.Context, req *connect.Request[exportv1.ExportCurlRequest]) (*connect.Response[exportv1.ExportCurlResponse], error) {
	h.logger.Info("Received ExportCurl request",
		"workspace_id", req.Msg.WorkspaceId,
		"http_ids_count", len(req.Msg.HttpIds))

	// Convert protobuf request to internal request model
	curlReq, err := convertToExportCurlRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Call the service to process the cURL export
	response, err := h.service.ExportCurl(ctx, curlReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Convert internal response to protobuf response
	protoResp := &exportv1.ExportCurlResponse{
		Data: response.Data,
	}

	h.logger.Info("ExportCurl completed successfully",
		"workspace_id", req.Msg.WorkspaceId,
		"curl_commands_length", len(protoResp.Data))

	return connect.NewResponse(protoResp), nil
}

// Private conversion functions

// convertToExportRequest converts protobuf request to internal request model
func convertToExportRequest(msg *exportv1.ExportRequest) (*ExportRequest, error) {
	// Parse workspace ID
	workspaceID, err := idwrap.NewFromBytes(msg.WorkspaceId)
	if err != nil {
		return nil, NewValidationError("workspaceId", err.Error())
	}

	// Convert file IDs
	fileIDs := make([]idwrap.IDWrap, 0, len(msg.FileIds))
	for _, fileIdBytes := range msg.FileIds {
		fileID, err := idwrap.NewFromBytes(fileIdBytes)
		if err != nil {
			return nil, NewValidationError("fileIds", err.Error())
		}
		fileIDs = append(fileIDs, fileID)
	}

	// Default format is YAML for standard Export RPC
	format := ExportFormat_YAML

	return &ExportRequest{
		WorkspaceID: workspaceID,
		FileIDs:     fileIDs,
		Format:      format,
		Simplified:  false,
	}, nil
}

// convertToExportCurlRequest converts protobuf cURL request to internal request model
func convertToExportCurlRequest(msg *exportv1.ExportCurlRequest) (*ExportCurlRequest, error) {
	// Parse workspace ID
	workspaceID, err := idwrap.NewFromBytes(msg.WorkspaceId)
	if err != nil {
		return nil, NewValidationError("workspaceId", err.Error())
	}

	// Convert HTTP IDs
	httpIDs := make([]idwrap.IDWrap, 0, len(msg.HttpIds))
	for _, httpIdBytes := range msg.HttpIds {
		httpID, err := idwrap.NewFromBytes(httpIdBytes)
		if err != nil {
			return nil, NewValidationError("httpIds", err.Error())
		}
		httpIDs = append(httpIDs, httpID)
	}

	return &ExportCurlRequest{
		WorkspaceID: workspaceID,
		HTTPIDs:     httpIDs,
	}, nil
}

// convertToExportResponse converts internal response to protobuf response model
func convertToExportResponse(resp *ExportResponse) (*exportv1.ExportResponse, error) {
	return &exportv1.ExportResponse{
		Name: resp.Name,
		Data: resp.Data,
	}, nil
}

// handleServiceError converts service errors to appropriate Connect errors
func handleServiceError(err error) error {
	if err == nil {
		return connect.NewError(connect.CodeInternal, NewValidationError("service_error", "nil error provided"))
	}

	switch {
	case IsValidationError(err):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, ErrWorkspaceNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ErrPermissionDenied):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, ErrExportFailed):
		return connect.NewError(connect.CodeInternal, err)
	case errors.Is(err, ErrNoDataFound) || errors.Is(err, sql.ErrNoRows):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ErrUnsupportedFormat):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, ErrTimeout) || errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
