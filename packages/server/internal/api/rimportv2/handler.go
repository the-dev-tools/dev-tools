package rimportv2

import (
	"context"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"the-dev-tools/server/pkg/idwrap"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"
	"the-dev-tools/spec/dist/buf/go/api/import/v1/importv1connect"
)

// ImportRPC implements the Connect RPC interface for HAR import
type ImportRPC struct {
	service *Service
	logger  *slog.Logger
}

// NewImportRPC creates a new ImportRPC handler
func NewImportRPC(service *Service, logger *slog.Logger) *ImportRPC {
	return &ImportRPC{
		service: service,
		logger:  logger,
	}
}

// Import implements the Import RPC method from the TypeSpec interface
func (h *ImportRPC) Import(ctx context.Context, req *connect.Request[apiv1.ImportRequest]) (*connect.Response[apiv1.ImportResponse], error) {
	h.logger.Info("Received Import RPC request",
		"workspace_id", req.Msg.WorkspaceId,
		"name", req.Msg.Name,
		"data_size", len(req.Msg.Data))

	// Convert protobuf request to internal request model
	importReq, err := h.convertToImportRequest(req.Msg)
	if err != nil {
		h.logger.Error("Failed to convert request", "error", err)
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Call the service to process the import
	response, err := h.service.Import(ctx, importReq)
	if err != nil {
		h.logger.Error("Import failed", "error", err)
		return h.handleServiceError(err)
	}

	// Convert internal response to protobuf response
	protoResp, err := h.convertToImportResponse(response)
	if err != nil {
		h.logger.Error("Failed to convert response", "error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	h.logger.Info("Import completed successfully",
		"missing_data", protoResp.MissingData,
		"domains", len(protoResp.Domains))

	return connect.NewResponse(protoResp), nil
}

// convertToImportRequest converts protobuf request to internal request model
func (h *ImportRPC) convertToImportRequest(msg *apiv1.ImportRequest) (*ImportRequest, error) {
	// Parse workspace ID
	workspaceID, err := idwrap.NewFromBytes(msg.WorkspaceId)
	if err != nil {
		return nil, NewValidationErrorWithCause("workspaceId", string(msg.WorkspaceId), err)
	}

	// Convert domain data
	domainData := make([]ImportDomainData, len(msg.DomainData))
	for i, dd := range msg.DomainData {
		domainData[i] = ImportDomainData{
			Enabled:  dd.Enabled,
			Domain:   dd.Domain,
			Variable: dd.Variable,
		}
	}

	return &ImportRequest{
		WorkspaceID: workspaceID,
		Name:        msg.Name,
		Data:        msg.Data,
		TextData:    msg.TextData,
		DomainData:  domainData,
	}, nil
}

// convertToImportResponse converts internal response to protobuf response model
func (h *ImportRPC) convertToImportResponse(resp *ImportResponse) (*apiv1.ImportResponse, error) {
	return &apiv1.ImportResponse{
		MissingData: apiv1.ImportMissingDataKind(resp.MissingData),
		Domains:     resp.Domains,
	}, nil
}

// handleServiceError converts service errors to appropriate Connect errors
func (h *ImportRPC) handleServiceError(err error) (*connect.Response[apiv1.ImportResponse], error) {
	switch {
	case IsValidationError(err):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case err == ErrWorkspaceNotFound:
		return nil, connect.NewError(connect.CodeNotFound, err)
	case err == ErrPermissionDenied:
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	case err == ErrInvalidHARFormat:
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case IsStorageError(err):
		h.logger.Error("Storage error", "error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	case IsHARProcessingError(err):
		h.logger.Error("HAR processing error", "error", err)
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	default:
		h.logger.Error("Unexpected error", "error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}
}

// Error type checking functions

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// IsStorageError checks if the error is a storage error
func IsStorageError(err error) bool {
	_, ok := err.(*StorageError)
	return ok
}

// IsHARProcessingError checks if the error is a HAR processing error
func IsHARProcessingError(err error) bool {
	_, ok := err.(*HARProcessingError)
	return ok
}

// RegisterImportRPC registers the ImportRPC handler with the HTTP mux
func RegisterImportRPC(mux *http.ServeMux, rpc *ImportRPC) {
	path, handler := importv1connect.NewImportServiceHandler(rpc)
	mux.Handle(path, handler)
}

// ImportServiceOption represents an option for configuring the ImportRPC
type ImportServiceOption func(*ImportRPC)

// WithLogger sets the logger for the ImportRPC
func WithLogger(logger *slog.Logger) ImportServiceOption {
	return func(rpc *ImportRPC) {
		rpc.logger = logger
	}
}

// NewImportRPCWithOptions creates a new ImportRPC with options
func NewImportRPCWithOptions(service *Service, opts ...ImportServiceOption) *ImportRPC {
	rpc := &ImportRPC{
		service: service,
		logger:  slog.Default(),
	}

	for _, opt := range opts {
		opt(rpc)
	}

	return rpc
}