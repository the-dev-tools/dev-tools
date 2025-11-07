package rimportv2

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"
	"the-dev-tools/spec/dist/buf/go/api/import/v1/importv1connect"
)

// ImportV2RPC implements the Connect RPC interface for HAR import v2
type ImportV2RPC struct {
	db      *sql.DB
	service *Service
	logger  *slog.Logger
	ws      sworkspace.WorkspaceService
	us      suser.UserService
}

// NewImportV2RPC creates a new ImportV2RPC handler with all required dependencies
func NewImportV2RPC(
	db *sql.DB,
	ws sworkspace.WorkspaceService,
	us suser.UserService,
	httpService *shttp.HTTPService,
	flowService *sflow.FlowService,
	fileService *sfile.FileService,
	logger *slog.Logger,
) *ImportV2RPC {
	// Create the importer with modern service dependencies
	importer := NewImporter(httpService, flowService, fileService)

	// Create the validator for input validation
	validator := NewValidator(&us)

	// Create the main service with functional options
	service := NewService(importer, validator, WithLogger(logger))

	// Create and return the RPC handler
	return &ImportV2RPC{
		db:      db,
		service: service,
		logger:  logger,
		ws:      ws,
		us:      us,
	}
}

// CreateImportV2Service creates the service registration for rimportv2
// This follows the exact same pattern as rimport.CreateService function
func CreateImportV2Service(srv ImportV2RPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := importv1connect.NewImportServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// Import implements the Import RPC method from the TypeSpec interface
// This method delegates to the internal service after proper validation and setup
func (h *ImportV2RPC) Import(ctx context.Context, req *connect.Request[apiv1.ImportRequest]) (*connect.Response[apiv1.ImportResponse], error) {
	startTime := time.Now()

	h.logger.Info("Received ImportV2 RPC request",
		"workspace_id", req.Msg.WorkspaceId,
		"name", req.Msg.Name,
		"data_size", len(req.Msg.Data))

	// Convert protobuf request to internal request model
	importReq, err := convertToImportRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Call the service to process the import
	response, err := h.service.Import(ctx, importReq)
	if err != nil {
		return handleServiceError(err)
	}

	// Convert internal response to protobuf response
	protoResp, err := convertToImportResponse(response)
	if err != nil {
		h.logger.Error("Response conversion failed - unexpected internal error",
			"workspace_id", req.Msg.WorkspaceId,
			"missing_data", response.MissingData,
			"domains_count", len(response.Domains),
			"error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcDuration := time.Since(startTime)
	h.logger.Info("ImportV2 RPC completed successfully",
		"workspace_id", req.Msg.WorkspaceId,
		"missing_data", protoResp.MissingData,
		"domains", len(protoResp.Domains),
		"duration_ms", rpcDuration.Milliseconds())

	return connect.NewResponse(protoResp), nil
}

