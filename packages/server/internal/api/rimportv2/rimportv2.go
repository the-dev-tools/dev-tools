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

// ImportServices groups all service dependencies
type ImportServices struct {
	Workspace          sworkspace.WorkspaceService
	User               suser.UserService
	Http               *shttp.HTTPService
	Flow               *sflow.FlowService
	File               *sfile.FileService
	Env                senv.EnvironmentService
	Var                svar.VarService
	HttpHeader         shttp.HttpHeaderService
	HttpSearchParam    *shttp.HttpSearchParamService
	HttpBodyForm       *shttp.HttpBodyFormService
	HttpBodyUrlEncoded *shttp.HttpBodyUrlEncodedService
	HttpBodyRaw        *shttp.HttpBodyRawService
	HttpAssert         *shttp.HttpAssertService
	Node               *snode.NodeService
	NodeRequest        *snoderequest.NodeRequestService
	NodeNoop           *snodenoop.NodeNoopService
	Edge               *sedge.EdgeService
}

// ImportStreamers groups all event streams
type ImportStreamers struct {
	Flow               eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]
	Node               eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent]
	Edge               eventstream.SyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent]
	Noop               eventstream.SyncStreamer[rflowv2.NoOpTopic, rflowv2.NoOpEvent]
	Http               eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]
	HttpHeader         eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]
	HttpSearchParam    eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]
	HttpBodyForm       eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]
	HttpBodyUrlEncoded eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]
	HttpBodyRaw        eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]
	HttpAssert         eventstream.SyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent]
	File               eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
}

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
	logger *slog.Logger,
	services ImportServices,
	streamers ImportStreamers,
) *ImportV2RPC {
	// Create the importer with modern service dependencies
	importer := NewImporter(db,
		services.Http, services.Flow, services.File,
		services.HttpHeader, services.HttpSearchParam, services.HttpBodyForm, services.HttpBodyUrlEncoded, services.HttpBodyRaw,
		services.HttpAssert, services.Node, services.NodeRequest, services.NodeNoop, services.Edge,
		services.Env, services.Var)

	// Create the validator for input validation
	validator := NewValidator(&services.User)

	// Create the main service with functional options
	service := NewService(importer, validator,
		WithLogger(logger),
		WithHTTPService(services.Http),
	)

	// Create and return the RPC handler
	return &ImportV2RPC{
		db:                       db,
		service:                  service,
		Logger:                   logger,
		ws:                       services.Workspace,
		us:                       services.User,
		FlowStream:               streamers.Flow,
		NodeStream:               streamers.Node,
		EdgeStream:               streamers.Edge,
		NoopStream:               streamers.Noop,
		HttpStream:               streamers.Http,
		HttpHeaderStream:         streamers.HttpHeader,
		HttpSearchParamStream:    streamers.HttpSearchParam,
		HttpBodyFormStream:       streamers.HttpBodyForm,
		HttpBodyUrlEncodedStream: streamers.HttpBodyUrlEncoded,
		HttpBodyRawStream:        streamers.HttpBodyRaw,
		HttpAssertStream:         streamers.HttpAssert,
		FileStream:               streamers.File,

		// Exposed Services
		HttpService:               services.Http,
		FlowService:               services.Flow,
		FileService:               services.File,
		HttpHeaderService:         services.HttpHeader,
		HttpSearchParamService:    services.HttpSearchParam,
		HttpBodyFormService:       services.HttpBodyForm,
		HttpBodyUrlEncodedService: services.HttpBodyUrlEncoded,
		HttpBodyRawService:        services.HttpBodyRaw,
		HttpAssertService:         services.HttpAssert,
		NodeService:               services.Node,
		NodeRequestService:        services.NodeRequest,
		NodeNoopService:           services.NodeNoop,
		EdgeService:               services.Edge,
		EnvService:                services.Env,
		VarService:                services.Var,
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