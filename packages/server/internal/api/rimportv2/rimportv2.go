//nolint:revive // exported
package rimportv2

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"
	"the-dev-tools/spec/dist/buf/go/api/import/v1/importv1connect"

	"connectrpc.com/connect"
)

// ImportV2RPC implements the Connect RPC interface for HAR import v2
type ImportV2RPC struct {
	db      *sql.DB
	service *Service
	Logger  *slog.Logger
	ws      sworkspace.WorkspaceService
	us      suser.UserService

	// Streamers for real-time updates
	FlowStream               eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]
	NodeStream               eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent]
	EdgeStream               eventstream.SyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent]
	NoopStream               eventstream.SyncStreamer[rflowv2.NoOpTopic, rflowv2.NoOpEvent]
	HttpStream               eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]
	HttpHeaderStream         eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]
	HttpSearchParamStream    eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]
	HttpBodyFormStream       eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]
	HttpBodyUrlEncodedStream eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]
	HttpBodyRawStream        eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]
	HttpAssertStream         eventstream.SyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent]
	FileStream               eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]

	// Services exposed for testing
	HttpService               *shttp.HTTPService
	FlowService               *sflow.FlowService
	FileService               *sfile.FileService
	HttpHeaderService         shttp.HttpHeaderService
	HttpSearchParamService    *shttp.HttpSearchParamService
	HttpBodyFormService       *shttp.HttpBodyFormService
	HttpBodyUrlEncodedService *shttp.HttpBodyUrlEncodedService
	HttpBodyRawService        *shttp.HttpBodyRawService
	HttpAssertService         *shttp.HttpAssertService
	NodeService               *snode.NodeService
	NodeRequestService        *snoderequest.NodeRequestService
	NodeNoopService           *snodenoop.NodeNoopService
	EdgeService               *sedge.EdgeService
	EnvService                senv.EnvironmentService
	VarService                svar.VarService
}

// NewImportV2RPC creates a new ImportV2RPC handler with all required dependencies
func NewImportV2RPC(
	db *sql.DB,
	ws sworkspace.WorkspaceService,
	us suser.UserService,
	httpService *shttp.HTTPService,
	flowService *sflow.FlowService,
	fileService *sfile.FileService,
	// Child entity services
	httpHeaderService shttp.HttpHeaderService,
	httpSearchParamService *shttp.HttpSearchParamService,
	httpBodyFormService *shttp.HttpBodyFormService,
	httpBodyUrlEncodedService *shttp.HttpBodyUrlEncodedService,
	bodyService *shttp.HttpBodyRawService,
	httpAssertService *shttp.HttpAssertService,
	nodeService *snode.NodeService,
	nodeRequestService *snoderequest.NodeRequestService,
	nodeNoopService *snodenoop.NodeNoopService,
	edgeService *sedge.EdgeService,
	envService senv.EnvironmentService,
	varService svar.VarService,
	logger *slog.Logger,
	// Streamers
	flowStream eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent],
	nodeStream eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent],
	edgeStream eventstream.SyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent],
	noopStream eventstream.SyncStreamer[rflowv2.NoOpTopic, rflowv2.NoOpEvent],
	stream eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent],
	httpHeaderStream eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent],
	httpSearchParamStream eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent],
	httpBodyFormStream eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent],
	httpBodyUrlEncodedStream eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent],
	httpBodyRawStream eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent],
	httpAssertStream eventstream.SyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent],
	fileStream eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent],
) *ImportV2RPC {
	// Create the importer with modern service dependencies
	importer := NewImporter(db, httpService, flowService, fileService,
		httpHeaderService, httpSearchParamService, httpBodyFormService, httpBodyUrlEncodedService, bodyService,
		httpAssertService, nodeService, nodeRequestService, nodeNoopService, edgeService, envService, varService)

	// Create the validator for input validation
	validator := NewValidator(&us)

	// Create the main service with functional options
	service := NewService(importer, validator,
		WithLogger(logger),
		WithHTTPService(httpService),
	)

	// Create and return the RPC handler
	return &ImportV2RPC{
		db:                       db,
		service:                  service,
		Logger:                   logger,
		ws:                       ws,
		us:                       us,
		FlowStream:               flowStream,
		NodeStream:               nodeStream,
		EdgeStream:               edgeStream,
		NoopStream:               noopStream,
		HttpStream:               stream,
		HttpHeaderStream:         httpHeaderStream,
		HttpSearchParamStream:    httpSearchParamStream,
		HttpBodyFormStream:       httpBodyFormStream,
		HttpBodyUrlEncodedStream: httpBodyUrlEncodedStream,
		HttpBodyRawStream:        httpBodyRawStream,
		HttpAssertStream:         httpAssertStream,
		FileStream:               fileStream,

		// Exposed Services
		HttpService:               httpService,
		FlowService:               flowService,
		FileService:               fileService,
		HttpHeaderService:         httpHeaderService,
		HttpSearchParamService:    httpSearchParamService,
		HttpBodyFormService:       httpBodyFormService,
		HttpBodyUrlEncodedService: httpBodyUrlEncodedService,
		HttpBodyRawService:        bodyService,
		HttpAssertService:         httpAssertService,
		NodeService:               nodeService,
		NodeRequestService:        nodeRequestService,
		NodeNoopService:           nodeNoopService,
		EdgeService:               edgeService,
		EnvService:                envService,
		VarService:                varService,
	}
}

// CreateImportV2Service creates the service registration for rimportv2
// This follows the exact same pattern as rimport.CreateService function
func CreateImportV2Service(srv ImportV2RPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := importv1connect.NewImportServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// ImportUnifiedInternal exposes the internal unified import logic for other server components
func (h *ImportV2RPC) ImportUnifiedInternal(ctx context.Context, req *ImportRequest) (*ImportResults, error) {
	return h.service.ImportUnified(ctx, req)
}

// Import implements the Import RPC method from the TypeSpec interface
// This method delegates to the internal service after proper validation and setup
func (h *ImportV2RPC) Import(ctx context.Context, req *connect.Request[apiv1.ImportRequest]) (*connect.Response[apiv1.ImportResponse], error) {
	startTime := time.Now()

	h.Logger.Info("Received ImportV2 RPC request",
		"workspace_id", req.Msg.WorkspaceId,
		"name", req.Msg.Name,
		"data_size", len(req.Msg.Data))

	// Convert protobuf request to internal request model
	importReq, err := convertToImportRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Call the service to process the import
	results, err := h.service.ImportUnified(ctx, importReq)
	if err != nil {
		return handleServiceError(err)
	}

	// Publish events for real-time sync ONLY if storage occurred (no missing data)
	if results.MissingData == ImportMissingDataKind_UNSPECIFIED {
		h.publishEvents(ctx, results)
	}

	// Convert internal response to protobuf response
	protoResp, err := convertToImportResponse(results)
	if err != nil {
		h.Logger.Error("Response conversion failed - unexpected internal error",
			"workspace_id", req.Msg.WorkspaceId,
			"missing_data", results.MissingData,
			"domains_count", len(results.Domains),
			"error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcDuration := time.Since(startTime)
	h.Logger.Info("ImportV2 RPC completed successfully",
		"workspace_id", req.Msg.WorkspaceId,
		"missing_data", protoResp.MissingData,
		"domains", len(protoResp.Domains),
		"duration_ms", rpcDuration.Milliseconds())

	return connect.NewResponse(protoResp), nil
}