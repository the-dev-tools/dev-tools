//nolint:revive // exported
package rimportv2

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
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
	Var                senv.VariableService
	HttpHeader         shttp.HttpHeaderService
	HttpSearchParam    *shttp.HttpSearchParamService
	HttpBodyForm       *shttp.HttpBodyFormService
	HttpBodyUrlEncoded *shttp.HttpBodyUrlEncodedService
	HttpBodyRaw        *shttp.HttpBodyRawService
	HttpAssert         *shttp.HttpAssertService
	Node               *sflow.NodeService
	NodeRequest        *sflow.NodeRequestService
	Edge               *sflow.EdgeService
}

func (s *ImportServices) Validate() error {
	// Http is a pointer to struct in DefaultImporter
	if s.Http == nil { return fmt.Errorf("http service is required") }
	if s.Flow == nil { return fmt.Errorf("flow service is required") }
	if s.File == nil { return fmt.Errorf("file service is required") }
	if s.HttpSearchParam == nil { return fmt.Errorf("http search param service is required") }
	if s.HttpBodyForm == nil { return fmt.Errorf("http body form service is required") }
	if s.HttpBodyUrlEncoded == nil { return fmt.Errorf("http body url encoded service is required") }
	if s.HttpBodyRaw == nil { return fmt.Errorf("http body raw service is required") }
	if s.HttpAssert == nil { return fmt.Errorf("http assert service is required") }
	if s.Node == nil { return fmt.Errorf("node service is required") }
	if s.NodeRequest == nil { return fmt.Errorf("node request service is required") }
	if s.Edge == nil { return fmt.Errorf("edge service is required") }
	return nil
}

// ImportStreamers groups all event streams
type ImportStreamers struct {
	Flow               eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]
	Node               eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent]
	Edge               eventstream.SyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent]
	Http               eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]
	HttpHeader         eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]
	HttpSearchParam    eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]
	HttpBodyForm       eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]
	HttpBodyUrlEncoded eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]
	HttpBodyRaw        eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]
	HttpAssert         eventstream.SyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent]
	File               eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
	Env                eventstream.SyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent]
	EnvVar             eventstream.SyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent]
}

func (s *ImportStreamers) Validate() error {
	if s.Flow == nil { return fmt.Errorf("flow stream is required") }
	if s.Http == nil { return fmt.Errorf("http stream is required") }
	if s.File == nil { return fmt.Errorf("file stream is required") }
	return nil
}

type ImportV2Deps struct {
	DB         *sql.DB
	Logger     *slog.Logger
	Services   ImportServices
	Readers    ImportV2Readers
	Streamers  ImportStreamers
}

type ImportV2Readers struct {
	Workspace *sworkspace.WorkspaceReader
	User      *sworkspace.UserReader
}

func (r *ImportV2Readers) Validate() error {
	if r.Workspace == nil { return fmt.Errorf("workspace reader is required") }
	if r.User == nil { return fmt.Errorf("user reader is required") }
	return nil
}

func (d *ImportV2Deps) Validate() error {
	if d.DB == nil { return fmt.Errorf("db is required") }
	if d.Logger == nil { return fmt.Errorf("logger is required") }
	if err := d.Services.Validate(); err != nil { return err }
	if err := d.Readers.Validate(); err != nil { return err }
	if err := d.Streamers.Validate(); err != nil { return err }
	return nil
}

// ImportV2RPC implements the Connect RPC interface for HAR import v2
type ImportV2RPC struct {
	db       *sql.DB
	service  *Service
	Logger   *slog.Logger
	ws       sworkspace.WorkspaceService
	us       suser.UserService
	importMu sync.Mutex

	wsReader *sworkspace.WorkspaceReader
	userReader *sworkspace.UserReader

	// Streamers for real-time updates
	FlowStream               eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]
	NodeStream               eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent]
	EdgeStream               eventstream.SyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent]
	HttpStream               eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]
	HttpHeaderStream         eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]
	HttpSearchParamStream    eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]
	HttpBodyFormStream       eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]
	HttpBodyUrlEncodedStream eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]
	HttpBodyRawStream        eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]
	HttpAssertStream         eventstream.SyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent]
	FileStream               eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
	EnvStream                eventstream.SyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent]
	EnvVarStream             eventstream.SyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent]

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
	NodeService               *sflow.NodeService
	NodeRequestService        *sflow.NodeRequestService
	EdgeService               *sflow.EdgeService
	EnvService                senv.EnvironmentService
	VarService                senv.VariableService
}

// NewImportV2RPC creates a new ImportV2RPC handler with all required dependencies
func NewImportV2RPC(deps ImportV2Deps) *ImportV2RPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("ImportV2 Deps validation failed: %v", err))
	}

	// Create the importer with modern service dependencies
	importer := NewImporter(deps.DB,
		deps.Services.Http, deps.Services.Flow, deps.Services.File,
		deps.Services.HttpHeader, deps.Services.HttpSearchParam, deps.Services.HttpBodyForm, deps.Services.HttpBodyUrlEncoded, deps.Services.HttpBodyRaw,
		deps.Services.HttpAssert, deps.Services.Node, deps.Services.NodeRequest, deps.Services.Edge,
		deps.Services.Env, deps.Services.Var)

	// Create the validator for input validation
	validator := NewValidator(&deps.Services.User, deps.Readers.User)

	// Create the main service with functional options
	service := NewService(importer, validator,
		WithLogger(deps.Logger),
		WithHTTPService(deps.Services.Http),
	)

	// Create and return the RPC handler
	return &ImportV2RPC{
		db:                       deps.DB,
		service:                  service,
		Logger:                   deps.Logger,
		ws:                       deps.Services.Workspace,
		us:                       deps.Services.User,
		wsReader:                 deps.Readers.Workspace,
		userReader:               deps.Readers.User,
		FlowStream:               deps.Streamers.Flow,
		NodeStream:               deps.Streamers.Node,
		EdgeStream:               deps.Streamers.Edge,
		HttpStream:               deps.Streamers.Http,
		HttpHeaderStream:         deps.Streamers.HttpHeader,
		HttpSearchParamStream:    deps.Streamers.HttpSearchParam,
		HttpBodyFormStream:       deps.Streamers.HttpBodyForm,
		HttpBodyUrlEncodedStream: deps.Streamers.HttpBodyUrlEncoded,
		HttpBodyRawStream:        deps.Streamers.HttpBodyRaw,
		HttpAssertStream:         deps.Streamers.HttpAssert,
		FileStream:               deps.Streamers.File,
		EnvStream:                deps.Streamers.Env,
		EnvVarStream:             deps.Streamers.EnvVar,

		// Exposed Services
		HttpService:               deps.Services.Http,
		FlowService:               deps.Services.Flow,
		FileService:               deps.Services.File,
		HttpHeaderService:         deps.Services.HttpHeader,
		HttpSearchParamService:    deps.Services.HttpSearchParam,
		HttpBodyFormService:       deps.Services.HttpBodyForm,
		HttpBodyUrlEncodedService: deps.Services.HttpBodyUrlEncoded,
		HttpBodyRawService:        deps.Services.HttpBodyRaw,
		HttpAssertService:         deps.Services.HttpAssert,
		NodeService:               deps.Services.Node,
		NodeRequestService:        deps.Services.NodeRequest,
		EdgeService:               deps.Services.Edge,
		EnvService:                deps.Services.Env,
		VarService:                deps.Services.Var,
	}
}

// CreateImportV2Service creates the service registration for rimportv2
// This follows the exact same pattern as rimport.CreateService function
func CreateImportV2Service(srv *ImportV2RPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := importv1connect.NewImportServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// ImportUnifiedInternal exposes the internal unified import logic for other server components
func (h *ImportV2RPC) ImportUnifiedInternal(ctx context.Context, req *ImportRequest) (*ImportResults, error) {
	return h.service.ImportUnified(ctx, req)
}

// Import implements the Import RPC method from the TypeSpec interface
// This method delegates to the internal service after proper validation and setup
func (h *ImportV2RPC) Import(ctx context.Context, req *connect.Request[apiv1.ImportRequest]) (*connect.Response[apiv1.ImportResponse], error) {
	h.importMu.Lock()
	defer h.importMu.Unlock()

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