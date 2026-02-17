//nolint:revive // exported
package rexportv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	exportv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/export/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/export/v1/exportv1connect"

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

type ExportV2Deps struct {
	DB        *sql.DB
	Queries   *gen.Queries
	Workspace sworkspace.WorkspaceService
	User      suser.UserService
	Http      *shttp.HTTPService
	Flow      *sflow.FlowService
	File      *sfile.FileService
	Logger    *slog.Logger
}

func (d *ExportV2Deps) Validate() error {
	if d.DB == nil {
		return fmt.Errorf("db is required")
	}
	if d.Queries == nil {
		return fmt.Errorf("queries is required")
	}
	if d.Http == nil {
		return fmt.Errorf("http service is required")
	}
	if d.Flow == nil {
		return fmt.Errorf("flow service is required")
	}
	if d.File == nil {
		return fmt.Errorf("file service is required")
	}
	if d.Logger == nil {
		return fmt.Errorf("logger is required")
	}
	return nil
}

// NewExportV2RPC creates a new ExportV2RPC handler with modern services
func NewExportV2RPC(deps ExportV2Deps) *ExportV2RPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("ExportV2 Deps validation failed: %v", err))
	}

	// Create IOWorkspaceService
	ioWorkspaceService := ioworkspace.New(deps.Queries, deps.Logger)

	// Create simple storage with modern services
	storage := NewStorage(&deps.Workspace, deps.Http, deps.Flow, deps.File)

	// Create simple exporter with IOWorkspaceService
	exporter := NewExporter(deps.Http, deps.Flow, deps.File, ioWorkspaceService)

	// Create simple validator
	validator := NewValidator(&deps.User)

	// Create the main service
	service := NewService(exporter, validator, storage)

	return &ExportV2RPC{
		db:      deps.DB,
		service: service,
		logger:  deps.Logger,
		ws:      deps.Workspace,
		us:      deps.User,
	}
}

// CreateExportV2Service creates the service registration for rexportv2
func CreateExportV2Service(srv ExportV2RPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := exportv1connect.NewExportServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// Export implements the Export RPC method
//
//nolint:nopermskip // permission check delegated to service.Export → validator.ValidateWorkspaceAccess
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
//
//nolint:nopermskip // permission check delegated to service.ExportCurl → validator.ValidateWorkspaceAccess
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
