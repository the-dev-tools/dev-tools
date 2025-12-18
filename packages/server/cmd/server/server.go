//nolint:revive // exported
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"connectrpc.com/connect"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/tursolocal"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/middleware/mwcompress"
	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/internal/api/rexportv2"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhealth"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/api/rimportv2"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/internal/api/rreference"

	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"
)

// workspaceImporterAdapter implements rflowv2.WorkspaceImporter using rimportv2 service
type workspaceImporterAdapter struct {
	importService *rimportv2.ImportV2RPC
}

func (w *workspaceImporterAdapter) ImportWorkspaceFromYAML(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*rflowv2.ImportResults, error) {
	req := &rimportv2.ImportRequest{
		WorkspaceID: workspaceID,
		Name:        "Imported Flow",
		Data:        data,
	}

	res, err := w.importService.ImportUnifiedInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	flowsCreated := 0
	if res.Flow != nil {
		flowsCreated = 1
	}

	return &rflowv2.ImportResults{
		WorkspaceID:     workspaceID,
		HTTPReqsCreated: len(res.HTTPReqs),
		FilesCreated:    len(res.Files),
		FlowsCreated:    flowsCreated,
		NodesCreated:    len(res.Nodes),
	}, nil
}

func (w *workspaceImporterAdapter) ImportWorkspaceFromCurl(ctx context.Context, curlData []byte, workspaceID idwrap.IDWrap) (*rflowv2.ImportResults, error) {
	// ImportUnified handles format detection automatically
	return w.ImportWorkspaceFromYAML(ctx, curlData, workspaceID)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	logger := setupLogger()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		return errors.New("HMAC_SECRET env var is required")
	}

	currentDB, dbCloseFunc, err := setupDB(ctx)
	if err != nil {
		return err
	}
	defer dbCloseFunc()

	queries, err := gen.Prepare(ctx, currentDB)
	if err != nil {
		return err
	}

	// Initialize Services
	workspaceService := sworkspace.NewWorkspaceService(queries)
	workspaceUserService := sworkspace.NewUserService(queries)
	userService := suser.New(queries)

	httpBodyRawService := shttp.NewHttpBodyRawService(queries)
	variableService := senv.NewVariableService(queries, logger)
	environmentService := senv.NewEnvironmentService(queries, logger)
	httpService := shttp.New(queries, logger)

	// HTTP child entity services
	httpHeaderService := shttp.NewHttpHeaderService(queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(queries)
	httpAssertService := shttp.NewHttpAssertService(queries)
	httpResponseService := shttp.NewHttpResponseService(queries)

	// File Service
	fileService := sfile.New(queries, logger)

	// Flow
	flowService := sflow.NewFlowService(queries)
	flowEdgeService := sflow.NewEdgeService(queries)
	flowVariableService := sflow.NewFlowVariableService(queries)

	// nodes
	flowNodeService := sflow.NewNodeService(queries)
	flowNodeRequestSevice := sflow.NewNodeRequestService(queries)
	flowNodeForService := sflow.NewNodeForService(queries)
	flowNodeForeachService := sflow.NewNodeForEachService(queries)
	flowNodeConditionService := sflow.NewNodeIfService(queries)
	flowNodeNoOpService := sflow.NewNodeNoopService(queries)
	flowNodeNodeJsService := sflow.NewNodeJsService(queries)
	nodeExecutionService := sflow.NewNodeExecutionService(queries)

	// Initialize Streamers
	streamers := NewStreamers()
	defer streamers.Shutdown()

	var optionsCompress, optionsAuth, optionsAll []connect.HandlerOption
	optionsCompress = append(optionsCompress, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	optionsCompress = append(optionsCompress, connect.WithCompression("gzip", nil, nil))
	_, err = userService.GetUser(ctx, mwauth.LocalDummyID)
	if err != nil {
		if errors.Is(err, suser.ErrUserNotFound) {
			defaultUser := &muser.User{
				ID: mwauth.LocalDummyID,
			}
			err = userService.CreateUser(ctx, defaultUser)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	optionsAuth = make([]connect.HandlerOption, len(optionsCompress), len(optionsCompress)+1)
	copy(optionsAuth, optionsCompress)
	optionsAuth = append(optionsAuth, connect.WithInterceptors(mwauth.NewAuthInterceptor()))
	optionsAll = make([]connect.HandlerOption, len(optionsAuth), len(optionsAuth)+len(optionsCompress))
	copy(optionsAll, optionsAuth)
	optionsAll = append(optionsAll, optionsCompress...)

	// Services Connect RPC
	newServiceManager := NewServiceManager(30)

	healthSrv := rhealth.New()
	newServiceManager.AddService(rhealth.CreateService(healthSrv, optionsCompress))

	workspaceSrv := rworkspace.New(currentDB, workspaceService, workspaceUserService, userService, environmentService, streamers.Workspace, streamers.Environment)
	newServiceManager.AddService(rworkspace.CreateService(workspaceSrv, optionsAll))

	envSrv := renv.New(currentDB, environmentService, variableService, userService, workspaceService, streamers.Environment, streamers.EnvironmentVariable)
	newServiceManager.AddService(renv.CreateService(envSrv, optionsAll))

	// Create request resolver for HTTP delta resolution (shared with flow service)
	requestResolver := resolver.NewStandardResolver(
		&httpService,
		&httpHeaderService,
		httpSearchParamService,
		httpBodyRawService,
		httpBodyFormService,
		httpBodyUrlEncodedService,
		httpAssertService,
	)

	httpStreamers := &rhttp.HttpStreamers{
		Http:               streamers.Http,
		HttpHeader:         streamers.HttpHeader,
		HttpSearchParam:    streamers.HttpSearchParam,
		HttpBodyForm:       streamers.HttpBodyForm,
		HttpBodyUrlEncoded: streamers.HttpBodyUrlEncoded,
		HttpAssert:         streamers.HttpAssert,
		HttpVersion:        streamers.HttpVersion,
		HttpResponse:       streamers.HttpResponse,
		HttpResponseHeader: streamers.HttpResponseHeader,
		HttpResponseAssert: streamers.HttpResponseAssert,
		HttpBodyRaw:        streamers.HttpBodyRaw,
		Log:                streamers.Log,
	}

	httpReader := shttp.NewReader(currentDB, logger, &workspaceUserService)

	httpSrv := rhttp.New(
		currentDB, httpReader, httpService, userService, workspaceService, workspaceUserService, environmentService, variableService,
		httpBodyRawService, httpHeaderService, httpSearchParamService, httpBodyFormService, httpBodyUrlEncodedService,
		httpAssertService, httpResponseService, requestResolver,
		httpStreamers,
	)
	newServiceManager.AddService(rhttp.CreateService(httpSrv, optionsAll))

	// ImportV2 Service
	importV2Srv := rimportv2.NewImportV2RPC(
		currentDB,
		logger,
		rimportv2.ImportServices{
			Workspace:          workspaceService,
			User:               userService,
			Http:               &httpService,
			Flow:               &flowService,
			File:               fileService,
			Env:                environmentService,
			Var:                variableService,
			HttpHeader:         httpHeaderService,
			HttpSearchParam:    httpSearchParamService,
			HttpBodyForm:       httpBodyFormService,
			HttpBodyUrlEncoded: httpBodyUrlEncodedService,
			HttpBodyRaw:        httpBodyRawService,
			HttpAssert:         httpAssertService,
			Node:               &flowNodeService,
			NodeRequest:        &flowNodeRequestSevice,
			NodeNoop:           &flowNodeNoOpService,
			Edge:               &flowEdgeService,
		},
		rimportv2.ImportStreamers{
			Flow:               streamers.Flow,
			Node:               streamers.Node,
			Edge:               streamers.Edge,
			Noop:               streamers.NoOp,
			Http:               streamers.Http,
			HttpHeader:         streamers.HttpHeader,
			HttpSearchParam:    streamers.HttpSearchParam,
			HttpBodyForm:       streamers.HttpBodyForm,
			HttpBodyUrlEncoded: streamers.HttpBodyUrlEncoded,
			HttpBodyRaw:        streamers.HttpBodyRaw,
			HttpAssert:         streamers.HttpAssert,
			File:               streamers.File,
			Env:                streamers.Environment,
			EnvVar:             streamers.EnvironmentVariable,
		},
	)
	newServiceManager.AddService(rimportv2.CreateImportV2Service(*importV2Srv, optionsAll))

	// Create workspace importer adapter for flow service
	workspaceImporter := &workspaceImporterAdapter{
		importService: importV2Srv,
	}

	// Create JS executor client
	// Environment variables:
	//   - WORKER_MODE: "uds" (default) or "tcp"
	//   - WORKER_SOCKET_PATH: custom socket path (uds mode)
	//   - WORKER_URL: full URL (tcp mode, defaults to http://localhost:9090)
	var jsHTTPClient *http.Client
	var jsBaseURL string

	workerMode := os.Getenv("WORKER_MODE")
	if workerMode == "" {
		workerMode = api.ServerModeUDS
	}

	switch workerMode {
	case api.ServerModeTCP:
		jsHTTPClient = http.DefaultClient
		jsBaseURL = os.Getenv("WORKER_URL")
		if jsBaseURL == "" {
			jsBaseURL = "http://localhost:9090"
		}
		slog.Info("Connecting to worker-js via TCP", "url", jsBaseURL)
	default:
		workerSocketPath := os.Getenv("WORKER_SOCKET_PATH")
		if workerSocketPath == "" {
			workerSocketPath = api.DefaultWorkerSocketPath()
		}
		jsHTTPClient = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					dialer := net.Dialer{}
					return dialer.DialContext(ctx, "unix", workerSocketPath)
				},
			},
		}
		// NOTE: ConnectRPC requires an address even for Unix sockets.
		// Use placeholder since actual routing is via socket.
		jsBaseURL = "http://the-dev-tools:0"
		slog.Info("Connecting to worker-js via Unix socket", "path", workerSocketPath)
	}

	jsClient := node_js_executorv1connect.NewNodeJsExecutorServiceClient(
		jsHTTPClient,
		jsBaseURL,
	)

	workspaceReader := sworkspace.NewWorkspaceReader(currentDB)
	userReader := sworkspace.NewUserReader(currentDB)
	flowReader := sflow.NewFlowReader(currentDB)
	nodeReader := sflow.NewNodeReader(currentDB)
	flowNodeRequestReader := sflow.NewNodeRequestReader(currentDB)
	flowVariableReader := sflow.NewFlowVariableReader(currentDB)
	flowEdgeReader := sflow.NewEdgeReader(currentDB)
	nodeExecutionReader := sflow.NewNodeExecutionReader(currentDB)
	httpReader = shttp.NewReader(currentDB, logger, &workspaceUserService)
	httpResponseReader := shttp.NewHttpResponseReader(currentDB)
	envReader := senv.NewEnvReader(currentDB, logger)
	varReader := senv.NewVariableReader(currentDB, logger)

	flowSrvV2 := rflowv2.New(
		currentDB,
		workspaceReader,
		flowReader,
		nodeReader,
		envReader,
		httpReader,
		&workspaceService,
		&flowService,
		&flowEdgeService,
		&flowNodeService,
		&flowNodeRequestSevice,
		&flowNodeForService,
		&flowNodeForeachService,
		flowNodeConditionService,
		&flowNodeNoOpService,
		&flowNodeNodeJsService,
		&nodeExecutionService,
		&flowVariableService,
		&environmentService,
		&variableService,
		&httpService,
		httpBodyRawService,
		requestResolver,
		logger,
		workspaceImporter,
		httpResponseService,
		streamers.Flow,
		streamers.Node,
		streamers.Edge,
		streamers.FlowVariable,
		streamers.FlowVersion,
		streamers.NoOp,
		streamers.For,
		streamers.Condition,
		streamers.ForEach,
		streamers.Js,
		streamers.Execution,
		streamers.HttpResponse,
		streamers.HttpResponseHeader,
		streamers.HttpResponseAssert,
		streamers.Log,
		jsClient,
	)
	newServiceManager.AddService(rflowv2.CreateService(flowSrvV2, optionsAll))

	logSrv := rlog.New(streamers.Log)
	newServiceManager.AddService(rlog.CreateService(logSrv, optionsAll))

	// ExportV2 Service
	exportV2Srv := rexportv2.NewExportV2RPC(
		currentDB,
		queries,
		workspaceService,
		userService,
		&httpService,
		&flowService,
		fileService,
		logger,
	)
	newServiceManager.AddService(rexportv2.CreateExportV2Service(*exportV2Srv, optionsAll))

	fileSrv := rfile.New(currentDB, fileService, userService, workspaceService, streamers.File)
	newServiceManager.AddService(rfile.CreateService(fileSrv, optionsAll))

	// Reference Service
	refServiceRPC := rreference.NewReferenceServiceRPC(currentDB,
		userReader,
		workspaceReader,
		envReader,
		varReader,
		flowReader,
		nodeReader,
		flowNodeRequestReader,
		flowVariableReader,
		flowEdgeReader,
		nodeExecutionReader,
		httpResponseReader,
	)
	newServiceManager.AddService(rreference.CreateService(refServiceRPC, optionsAll))

	// Start services
	go func() {
		err := api.ListenServices(newServiceManager.GetServices(), port)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for signal
	<-sc
	return nil
}

type ServiceManager struct {
	s []api.Service
}

// size is not max size, but initial allocation size for the slice
func NewServiceManager(size int) *ServiceManager {
	return &ServiceManager{
		s: make([]api.Service, 0, size),
	}
}

func (sm *ServiceManager) AddService(s *api.Service, e error) {
	if e != nil {
		log.Fatalf("error: %v on %s", e, s.Path)
	}
	if s == nil {
		log.Fatalf("service is nil on %d", len(sm.s))
	}
	sm.s = append(sm.s, *s)
}

func (sm *ServiceManager) GetServices() []api.Service {
	return sm.s
}

func setupLogger() *slog.Logger {
	var logLevel slog.Level
	logLevelStr := os.Getenv("LOG_LEVEL")
	switch logLevelStr {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "INFO":
		logLevel = slog.LevelInfo
	case "WARNING":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelError
	}

	loggerHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})

	return slog.New(loggerHandler)
}

func setupDB(ctx context.Context) (*sql.DB, func(), error) {
	dbMode := os.Getenv("DB_MODE")
	if dbMode == "" {
		return nil, nil, errors.New("DB_MODE env var is required")
	}
	fmt.Println("DB_MODE: ", dbMode)

	switch dbMode {
	case devtoolsdb.LOCAL:
		return GetDBLocal(ctx)
	default:
		return nil, nil, errors.New("invalid db mode")
	}
}

func GetDBLocal(ctx context.Context) (*sql.DB, func(), error) {
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		return nil, nil, errors.New("DB_NAME env var is required")
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		return nil, nil, errors.New("DB_PATH env var is required")
	}
	encryptKey := os.Getenv("DB_ENCRYPTION_KEY")
	if encryptKey == "" {
		return nil, nil, errors.New("DB_ENCRYPT_KEY env var is required")
	}
	localDB, err := tursolocal.NewTursoLocal(ctx, dbName, dbPath, encryptKey)
	if err != nil {
		return nil, nil, err
	}
	cleanup := localDB.CleanupFunc
	if cleanup == nil {
		cleanup = func() {}
	}
	return localDB.WriteDB, cleanup, nil
}

type Streamers struct {
	Workspace           eventstream.SyncStreamer[rworkspace.WorkspaceTopic, rworkspace.WorkspaceEvent]
	Environment         eventstream.SyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent]
	EnvironmentVariable eventstream.SyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent]
	Log                 eventstream.SyncStreamer[rlog.LogTopic, rlog.LogEvent]
	Http                eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]
	HttpHeader          eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]
	HttpSearchParam     eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]
	HttpBodyForm        eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]
	HttpBodyUrlEncoded  eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]
	HttpAssert          eventstream.SyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent]
	HttpVersion         eventstream.SyncStreamer[rhttp.HttpVersionTopic, rhttp.HttpVersionEvent]
	HttpResponse        eventstream.SyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent]
	HttpResponseHeader  eventstream.SyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent]
	HttpResponseAssert  eventstream.SyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent]
	HttpBodyRaw         eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]
	Flow                eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]
	Node                eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent]
	Edge                eventstream.SyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent]
	FlowVariable        eventstream.SyncStreamer[rflowv2.FlowVariableTopic, rflowv2.FlowVariableEvent]
	FlowVersion         eventstream.SyncStreamer[rflowv2.FlowVersionTopic, rflowv2.FlowVersionEvent]
	NoOp                eventstream.SyncStreamer[rflowv2.NoOpTopic, rflowv2.NoOpEvent]
	For                 eventstream.SyncStreamer[rflowv2.ForTopic, rflowv2.ForEvent]
	Condition           eventstream.SyncStreamer[rflowv2.ConditionTopic, rflowv2.ConditionEvent]
	ForEach             eventstream.SyncStreamer[rflowv2.ForEachTopic, rflowv2.ForEachEvent]
	Js                  eventstream.SyncStreamer[rflowv2.JsTopic, rflowv2.JsEvent]
	Execution           eventstream.SyncStreamer[rflowv2.ExecutionTopic, rflowv2.ExecutionEvent]
	File                eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
}

func NewStreamers() *Streamers {
	return &Streamers{
		Workspace:           memory.NewInMemorySyncStreamer[rworkspace.WorkspaceTopic, rworkspace.WorkspaceEvent](),
		Environment:         memory.NewInMemorySyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent](),
		EnvironmentVariable: memory.NewInMemorySyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent](),
		Log:                 memory.NewInMemorySyncStreamer[rlog.LogTopic, rlog.LogEvent](),
		Http:                memory.NewInMemorySyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent](),
		HttpHeader:          memory.NewInMemorySyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent](),
		HttpSearchParam:     memory.NewInMemorySyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent](),
		HttpBodyForm:        memory.NewInMemorySyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent](),
		HttpBodyUrlEncoded:  memory.NewInMemorySyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent](),
		HttpAssert:          memory.NewInMemorySyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent](),
		HttpVersion:         memory.NewInMemorySyncStreamer[rhttp.HttpVersionTopic, rhttp.HttpVersionEvent](),
		HttpResponse:        memory.NewInMemorySyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent](),
		HttpResponseHeader:  memory.NewInMemorySyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent](),
		HttpResponseAssert:  memory.NewInMemorySyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent](),
		HttpBodyRaw:         memory.NewInMemorySyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent](),
		Flow:                memory.NewInMemorySyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent](),
		Node:                memory.NewInMemorySyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent](),
		Edge:                memory.NewInMemorySyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent](),
		FlowVariable:        memory.NewInMemorySyncStreamer[rflowv2.FlowVariableTopic, rflowv2.FlowVariableEvent](),
		FlowVersion:         memory.NewInMemorySyncStreamer[rflowv2.FlowVersionTopic, rflowv2.FlowVersionEvent](),
		NoOp:                memory.NewInMemorySyncStreamer[rflowv2.NoOpTopic, rflowv2.NoOpEvent](),
		For:                 memory.NewInMemorySyncStreamer[rflowv2.ForTopic, rflowv2.ForEvent](),
		Condition:           memory.NewInMemorySyncStreamer[rflowv2.ConditionTopic, rflowv2.ConditionEvent](),
		ForEach:             memory.NewInMemorySyncStreamer[rflowv2.ForEachTopic, rflowv2.ForEachEvent](),
		Js:                  memory.NewInMemorySyncStreamer[rflowv2.JsTopic, rflowv2.JsEvent](),
		Execution:           memory.NewInMemorySyncStreamer[rflowv2.ExecutionTopic, rflowv2.ExecutionEvent](),
		File:                memory.NewInMemorySyncStreamer[rfile.FileTopic, rfile.FileEvent](),
	}
}

func (s *Streamers) Shutdown() {
	s.Workspace.Shutdown()
	s.Environment.Shutdown()
	s.EnvironmentVariable.Shutdown()
	s.Log.Shutdown()
	s.Http.Shutdown()
	s.HttpHeader.Shutdown()
	s.HttpSearchParam.Shutdown()
	s.HttpBodyForm.Shutdown()
	s.HttpBodyUrlEncoded.Shutdown()
	s.HttpAssert.Shutdown()
	s.HttpVersion.Shutdown()
	s.HttpResponse.Shutdown()
	s.HttpResponseHeader.Shutdown()
	s.HttpResponseAssert.Shutdown()
	s.HttpBodyRaw.Shutdown()
	s.Flow.Shutdown()
	s.Node.Shutdown()
	s.Edge.Shutdown()
	s.FlowVariable.Shutdown()
	s.FlowVersion.Shutdown()
	s.NoOp.Shutdown()
	s.For.Shutdown()
	s.Condition.Shutdown()
	s.ForEach.Shutdown()
	s.Js.Shutdown()
	s.Execution.Shutdown()
	s.File.Shutdown()
}
