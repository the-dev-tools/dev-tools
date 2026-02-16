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
	"path/filepath"
	"syscall"
	"time"

	"connectrpc.com/connect"

	"github.com/the-dev-tools/dev-tools/packages/auth/authlib/jwks"
	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/tursolocal"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrations"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwcodec"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwcompress"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/renv"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rexportv2"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rflowv2"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhealth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rimportv2"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rlog"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rreference"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/ruser"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/credvault"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/streamregistry"
	envapiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/environment/v1"
	filesystemv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/file_system/v1"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
	httpv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1/auth_internalv1connect"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/workspace/v1"
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

	// Auth configuration
	// AUTH_MODE: "local" (default, single-user desktop/CLI) or "betterauth" (multi-user, self-hosted or hosted)
	authMode := os.Getenv("AUTH_MODE")
	if authMode == "" {
		authMode = "local"
	}

	// JWKS_URL: URL to fetch JWKS public keys for JWT validation (required if AUTH_MODE=betterauth)
	// Defaults to BETTERAUTH_URL + "/api/auth/jwks" if BETTERAUTH_URL is set.
	jwksURL := os.Getenv("JWKS_URL")
	if jwksURL == "" {
		if betterAuthURL := os.Getenv("BETTERAUTH_URL"); betterAuthURL != "" {
			jwksURL = betterAuthURL + "/api/auth/jwks"
		}
	}

	currentDB, dbCloseFunc, err := setupDB(ctx)
	if err != nil {
		return err
	}
	defer dbCloseFunc()

	// Initialize Queries
	queries := gen.New(currentDB)

	// Initialize Services
	workspaceService := sworkspace.NewWorkspaceService(queries)
	workspaceReader := sworkspace.NewWorkspaceReader(currentDB)

	workspaceUserService := sworkspace.NewUserService(queries)
	userReader := sworkspace.NewUserReader(currentDB)

	userService := suser.New(queries)
	// No dedicated userReader service struct yet, we use the reader from userService or just pass queries

	httpBodyRawService := shttp.NewHttpBodyRawService(queries)

	variableService := senv.NewVariableService(queries, logger)
	varReader := senv.NewVariableReader(currentDB, logger)

	environmentService := senv.NewEnvironmentService(queries, logger)
	envReader := senv.NewEnvReader(currentDB, logger)

	httpService := shttp.New(queries, logger)
	httpReader := shttp.NewReader(currentDB, logger, &workspaceUserService)

	// HTTP child entity services
	httpHeaderService := shttp.NewHttpHeaderService(queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(queries)
	httpAssertService := shttp.NewHttpAssertService(queries)
	httpResponseService := shttp.NewHttpResponseService(queries)
	httpResponseReader := shttp.NewHttpResponseReader(currentDB)

	// File Service
	fileService := sfile.New(queries, logger)

	// Credential Service
	vault := credvault.NewDefault()
	credentialService := scredential.NewCredentialService(queries, scredential.WithVault(vault))
	credentialReader := scredential.NewCredentialReader(currentDB, scredential.WithDecrypter(vault))

	// Flow
	flowService := sflow.NewFlowService(queries)
	flowReader := sflow.NewFlowReader(currentDB)

	flowEdgeService := sflow.NewEdgeService(queries)
	flowEdgeReader := sflow.NewEdgeReader(currentDB)

	flowVariableService := sflow.NewFlowVariableService(queries)
	flowVariableReader := sflow.NewFlowVariableReader(currentDB)

	// nodes
	flowNodeService := sflow.NewNodeService(queries)
	nodeReader := sflow.NewNodeReader(currentDB)

	flowNodeRequestSevice := sflow.NewNodeRequestService(queries)
	flowNodeRequestReader := sflow.NewNodeRequestReader(currentDB)

	flowNodeForService := sflow.NewNodeForService(queries)
	flowNodeForeachService := sflow.NewNodeForEachService(queries)
	flowNodeConditionService := sflow.NewNodeIfService(queries)
	flowNodeNodeJsService := sflow.NewNodeJsService(queries)
	flowNodeAIService := sflow.NewNodeAIService(queries)
	flowNodeAiProviderService := sflow.NewNodeAiProviderService(queries)
	flowNodeMemoryService := sflow.NewNodeMemoryService(queries)

	nodeExecutionService := sflow.NewNodeExecutionService(queries)
	nodeExecutionReader := sflow.NewNodeExecutionReader(currentDB)

	// Initialize Streamers
	streamers := NewStreamers()
	defer streamers.Shutdown()

	var optionsCompress, optionsAuth, optionsAll []connect.HandlerOption
	optionsCompress = append(optionsCompress, mwcodec.WithJSONCodec()) // Custom JSON codec that emits zero values
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

	// Choose auth interceptor based on AUTH_MODE
	// - "local": Single-user mode for desktop/CLI (no auth, dummy user)
	// - "betterauth": Multi-user mode with BetterAuth (self-hosted or hosted, JWKS JWT validation)
	var authInterceptor connect.Interceptor
	var authInternalClient auth_internalv1connect.AuthInternalServiceClient
	switch authMode {
	case "betterauth":
		if jwksURL == "" {
			return errors.New("JWKS_URL (or BETTERAUTH_URL) env var is required when AUTH_MODE=betterauth")
		}
		slog.Info("Using BetterAuth authentication mode (JWKS validation)", "jwks_url", jwksURL)
		provider, err := jwks.NewProvider(jwksURL)
		if err != nil {
			return fmt.Errorf("failed to create JWKS provider: %w", err)
		}
		provider.Start(ctx)
		authInterceptor = mwauth.NewBetterAuthInterceptor(provider.Keyfunc(), userService)

		betterAuthURL := os.Getenv("BETTERAUTH_URL")
		if betterAuthURL != "" {
			authInternalClient = auth_internalv1connect.NewAuthInternalServiceClient(
				&http.Client{Timeout: 30 * time.Second},
				betterAuthURL,
			)
		}
	default:
		slog.Info("Using local authentication mode")
		authInterceptor = mwauth.NewAuthInterceptor()
	}

	optionsAuth = append(optionsAuth, connect.WithInterceptors(authInterceptor))
	optionsAll = make([]connect.HandlerOption, len(optionsAuth), len(optionsAuth)+len(optionsCompress))
	copy(optionsAll, optionsAuth)
	optionsAll = append(optionsAll, optionsCompress...)

	// Services Connect RPC
	newServiceManager := NewServiceManager(30)

	healthSrv := rhealth.New()
	newServiceManager.AddService(rhealth.CreateService(healthSrv, optionsCompress))

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
		File:               streamers.File,
	}

	// Create stream registry for unified mutation event publishing
	registry := streamregistry.New()
	registerCascadeHandlers(registry, httpStreamers, streamers)

	workspaceSrv := rworkspace.New(rworkspace.WorkspaceServiceRPCDeps{
		DB: currentDB,
		Services: rworkspace.WorkspaceServiceRPCServices{
			Workspace:     workspaceService,
			WorkspaceUser: workspaceUserService,
			User:          userService,
			Env:           environmentService,
		},
		Readers: rworkspace.WorkspaceServiceRPCReaders{
			Workspace: workspaceReader,
			User:      userReader,
		},
		Streamers: rworkspace.WorkspaceServiceRPCStreamers{
			Workspace:   streamers.Workspace,
			Environment: streamers.Environment,
		},
		Publisher: registry,
	})
	newServiceManager.AddService(rworkspace.CreateService(workspaceSrv, optionsAll))

	userSrv := ruser.New(ruser.UserServiceRPCDeps{
		DB:                    currentDB,
		User:                  userService,
		Streamer:              streamers.User,
		LinkedAccountStreamer: streamers.LinkedAccount,
		AuthClient:            authInternalClient,
	})
	newServiceManager.AddService(ruser.CreateService(userSrv, optionsAll))

	envSrv := renv.New(renv.EnvRPCDeps{
		DB: currentDB,
		Services: renv.EnvRPCServices{
			Env:       environmentService,
			Variable:  variableService,
			User:      userService,
			Workspace: workspaceService,
		},
		Readers: renv.EnvRPCReaders{
			Env:      envReader,
			Variable: varReader,
		},
		Streamers: renv.EnvRPCStreamers{
			Env:      streamers.Environment,
			Variable: streamers.EnvironmentVariable,
		},
		Publisher: registry,
	})
	newServiceManager.AddService(renv.CreateService(envSrv, optionsAll))

	// Create request resolver for HTTP delta resolution (shared with flow service)
	// IMPORTANT: Resolvers should use Read-Only services for lookups
	requestResolver := resolver.NewStandardResolver(
		&httpService,
		&httpHeaderService,
		httpSearchParamService,
		httpBodyRawService,
		httpBodyFormService,
		httpBodyUrlEncodedService,
		httpAssertService,
	)

	httpSrv := rhttp.New(rhttp.HttpServiceRPCDeps{
		DB: currentDB,
		Readers: rhttp.HttpServiceRPCReaders{
			Http:      httpReader,
			User:      userReader,
			Workspace: workspaceReader,
		},
		Services: rhttp.HttpServiceRPCServices{
			Http:               httpService,
			User:               userService,
			Workspace:          workspaceService,
			WorkspaceUser:      workspaceUserService,
			Env:                environmentService,
			Variable:           variableService,
			HttpBodyRaw:        httpBodyRawService,
			HttpHeader:         httpHeaderService,
			HttpSearchParam:    httpSearchParamService,
			HttpBodyForm:       httpBodyFormService,
			HttpBodyUrlEncoded: httpBodyUrlEncodedService,
			HttpAssert:         httpAssertService,
			HttpResponse:       httpResponseService,
			File:               fileService,
		},
		Resolver:  requestResolver,
		Streamers: httpStreamers,
	})
	newServiceManager.AddService(rhttp.CreateService(httpSrv, optionsAll))

	// ImportV2 Service
	importV2Srv := rimportv2.NewImportV2RPC(rimportv2.ImportV2Deps{
		DB:     currentDB,
		Logger: logger,
		Services: rimportv2.ImportServices{
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
			Edge:               &flowEdgeService,
		},
		Readers: rimportv2.ImportV2Readers{
			Workspace: workspaceReader,
			User:      userReader,
		},
		Streamers: rimportv2.ImportStreamers{
			Flow:               streamers.Flow,
			Node:               streamers.Node,
			Edge:               streamers.Edge,
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
	})
	newServiceManager.AddService(rimportv2.CreateImportV2Service(importV2Srv, optionsAll))

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
					return api.DialWorker(ctx, workerSocketPath)
				},
			},
		}
		// NOTE: ConnectRPC requires an address even for Unix sockets.
		// Use placeholder since actual routing is via socket.
		jsBaseURL = "http://the-dev-tools:0"
		slog.Info("Connecting to worker-js via socket", "path", workerSocketPath)
	}

	jsClient := node_js_executorv1connect.NewNodeJsExecutorServiceClient(
		jsHTTPClient,
		jsBaseURL,
	)

	flowSrvV2 := rflowv2.New(rflowv2.FlowServiceV2Deps{
		DB: currentDB,
		Readers: rflowv2.FlowServiceV2Readers{
			Workspace:     workspaceReader,
			Flow:          flowReader,
			Node:          nodeReader,
			Env:           envReader,
			Http:          httpReader,
			Edge:          flowEdgeReader,
			NodeRequest:   flowNodeRequestReader,
			FlowVariable:  flowVariableReader,
			NodeExecution: nodeExecutionReader,
			HttpResponse:  httpResponseReader,
		},
		Services: rflowv2.FlowServiceV2Services{
			Workspace:     &workspaceService,
			Flow:          &flowService,
			Edge:          &flowEdgeService,
			Node:          &flowNodeService,
			NodeRequest:   &flowNodeRequestSevice,
			NodeFor:       &flowNodeForService,
			NodeForEach:   &flowNodeForeachService,
			NodeIf:        flowNodeConditionService,
			NodeJs:        &flowNodeNodeJsService,
			NodeAI:        &flowNodeAIService,
			NodeAiProvider: &flowNodeAiProviderService,
			NodeMemory:    &flowNodeMemoryService,
			NodeExecution: &nodeExecutionService,
			FlowVariable:  &flowVariableService,
			Env:           &environmentService,
			Var:           &variableService,
			Http:          &httpService,
			HttpBodyRaw:   httpBodyRawService,
			HttpResponse:  httpResponseService,
			File:          fileService,
			Importer:      workspaceImporter,
			Credential:    credentialService,
		},
		Streamers: rflowv2.FlowServiceV2Streamers{
			Flow:               streamers.Flow,
			Node:               streamers.Node,
			Edge:               streamers.Edge,
			Var:                streamers.FlowVariable,
			Version:            streamers.FlowVersion,
			For:                streamers.For,
			Condition:          streamers.Condition,
			ForEach:            streamers.ForEach,
			Js:                 streamers.Js,
			Ai:                 streamers.Ai,
			AiProvider:         streamers.AiProvider,
			Memory:             streamers.Memory,
			Execution:          streamers.Execution,
			HttpResponse:       streamers.HttpResponse,
			HttpResponseHeader: streamers.HttpResponseHeader,
			HttpResponseAssert: streamers.HttpResponseAssert,
			Log:                streamers.Log,
			File:               streamers.File,
		},
		Resolver: requestResolver,
		Logger:   logger,
		JsClient: jsClient,
	})
	newServiceManager.AddService(rflowv2.CreateService(flowSrvV2, optionsAll))

	logSrv := rlog.New(streamers.Log)
	newServiceManager.AddService(rlog.CreateService(logSrv, optionsAll))

	// ExportV2 Service
	exportV2Srv := rexportv2.NewExportV2RPC(rexportv2.ExportV2Deps{
		DB:        currentDB,
		Queries:   queries,
		Workspace: workspaceService,
		User:      userService,
		Http:      &httpService,
		Flow:      &flowService,
		File:      fileService,
		Logger:    logger,
	})
	newServiceManager.AddService(rexportv2.CreateExportV2Service(*exportV2Srv, optionsAll))

	fileSrv := rfile.New(rfile.FileServiceRPCDeps{
		DB: currentDB,
		Services: rfile.FileServiceRPCServices{
			File:      fileService,
			User:      userService,
			Workspace: workspaceService,
		},
		Stream:    streamers.File,
		Publisher: registry,
	})
	newServiceManager.AddService(rfile.CreateService(fileSrv, optionsAll))

	credentialSrv := rcredential.New(rcredential.CredentialRPCDeps{
		DB: currentDB,
		Services: rcredential.CredentialRPCServices{
			Credential: credentialService,
			User:       userService,
			Workspace:  workspaceService,
		},
		Readers: rcredential.CredentialRPCReaders{
			Credential: credentialReader,
		},
		Streamers: rcredential.CredentialRPCStreamers{
			Credential: streamers.Credential,
			OpenAi:     streamers.CredentialOpenAi,
			Gemini:     streamers.CredentialGemini,
			Anthropic:  streamers.CredentialAnthropic,
		},
		Publisher: registry,
	})
	newServiceManager.AddService(rcredential.CreateService(credentialSrv, optionsAll))

	// Reference Service
	refServiceRPC := rreference.NewReferenceServiceRPC(rreference.ReferenceServiceRPCDeps{
		DB: currentDB,
		Readers: rreference.ReferenceServiceRPCReaders{
			User:          userReader,
			Workspace:     workspaceReader,
			Env:           envReader,
			Variable:      varReader,
			Flow:          flowReader,
			Node:          nodeReader,
			NodeRequest:   flowNodeRequestReader,
			FlowVariable:  flowVariableReader,
			FlowEdge:      flowEdgeReader,
			NodeExecution: nodeExecutionReader,
			HttpResponse:  httpResponseReader,
		},
	})
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
	slog.Info("DB mode", "mode", dbMode)

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

	// Run database migrations before returning the connection.
	// Migrations are idempotent and track state in schema_migrations table.
	dbFilePath := filepath.Join(dbPath, dbName+".db")
	migrationCfg := migrations.Config{
		DatabasePath: dbFilePath,
		DataDir:      dbPath,
		Logger:       slog.Default(),
	}
	if err := migrations.Run(ctx, localDB.WriteDB, migrationCfg); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return localDB.WriteDB, cleanup, nil
}

type Streamers struct {
	User                eventstream.SyncStreamer[ruser.UserTopic, ruser.UserEvent]
	LinkedAccount       eventstream.SyncStreamer[ruser.LinkedAccountTopic, ruser.LinkedAccountEvent]
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
	For                 eventstream.SyncStreamer[rflowv2.ForTopic, rflowv2.ForEvent]
	Condition           eventstream.SyncStreamer[rflowv2.ConditionTopic, rflowv2.ConditionEvent]
	ForEach             eventstream.SyncStreamer[rflowv2.ForEachTopic, rflowv2.ForEachEvent]
	Js                  eventstream.SyncStreamer[rflowv2.JsTopic, rflowv2.JsEvent]
	Ai                  eventstream.SyncStreamer[rflowv2.AiTopic, rflowv2.AiEvent]
	AiProvider          eventstream.SyncStreamer[rflowv2.AiProviderTopic, rflowv2.AiProviderEvent]
	Memory              eventstream.SyncStreamer[rflowv2.MemoryTopic, rflowv2.MemoryEvent]
	Execution           eventstream.SyncStreamer[rflowv2.ExecutionTopic, rflowv2.ExecutionEvent]
	File                eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
	Credential          eventstream.SyncStreamer[rcredential.CredentialTopic, rcredential.CredentialEvent]
	CredentialOpenAi    eventstream.SyncStreamer[rcredential.CredentialOpenAiTopic, rcredential.CredentialOpenAiEvent]
	CredentialGemini    eventstream.SyncStreamer[rcredential.CredentialGeminiTopic, rcredential.CredentialGeminiEvent]
	CredentialAnthropic eventstream.SyncStreamer[rcredential.CredentialAnthropicTopic, rcredential.CredentialAnthropicEvent]
}

func NewStreamers() *Streamers {
	return &Streamers{
		User:                memory.NewInMemorySyncStreamer[ruser.UserTopic, ruser.UserEvent](),
		LinkedAccount:       memory.NewInMemorySyncStreamer[ruser.LinkedAccountTopic, ruser.LinkedAccountEvent](),
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
		For:                 memory.NewInMemorySyncStreamer[rflowv2.ForTopic, rflowv2.ForEvent](),
		Condition:           memory.NewInMemorySyncStreamer[rflowv2.ConditionTopic, rflowv2.ConditionEvent](),
		ForEach:             memory.NewInMemorySyncStreamer[rflowv2.ForEachTopic, rflowv2.ForEachEvent](),
		Js:                  memory.NewInMemorySyncStreamer[rflowv2.JsTopic, rflowv2.JsEvent](),
		Ai:                  memory.NewInMemorySyncStreamer[rflowv2.AiTopic, rflowv2.AiEvent](),
		AiProvider:          memory.NewInMemorySyncStreamer[rflowv2.AiProviderTopic, rflowv2.AiProviderEvent](),
		Memory:              memory.NewInMemorySyncStreamer[rflowv2.MemoryTopic, rflowv2.MemoryEvent](),
		Execution:           memory.NewInMemorySyncStreamer[rflowv2.ExecutionTopic, rflowv2.ExecutionEvent](),
		File:                memory.NewInMemorySyncStreamer[rfile.FileTopic, rfile.FileEvent](),
		Credential:          memory.NewInMemorySyncStreamer[rcredential.CredentialTopic, rcredential.CredentialEvent](),
		CredentialOpenAi:    memory.NewInMemorySyncStreamer[rcredential.CredentialOpenAiTopic, rcredential.CredentialOpenAiEvent](),
		CredentialGemini:    memory.NewInMemorySyncStreamer[rcredential.CredentialGeminiTopic, rcredential.CredentialGeminiEvent](),
		CredentialAnthropic: memory.NewInMemorySyncStreamer[rcredential.CredentialAnthropicTopic, rcredential.CredentialAnthropicEvent](),
	}
}

func (s *Streamers) Shutdown() {
	s.User.Shutdown()
	s.LinkedAccount.Shutdown()
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
	s.For.Shutdown()
	s.Condition.Shutdown()
	s.ForEach.Shutdown()
	s.Js.Shutdown()
	s.Ai.Shutdown()
	s.AiProvider.Shutdown()
	s.Memory.Shutdown()
	s.Execution.Shutdown()
	s.File.Shutdown()
	s.Credential.Shutdown()
	s.CredentialOpenAi.Shutdown()
	s.CredentialGemini.Shutdown()
	s.CredentialAnthropic.Shutdown()
}

// registerCascadeHandlers registers all handlers needed for cascade deletion events.
// This wires the streamregistry to the concrete streamers from rhttp, rflowv2, rfile, renv, and rworkspace.
func registerCascadeHandlers(registry *streamregistry.Registry, httpStreamers *rhttp.HttpStreamers, streamers *Streamers) {
	// Workspace entity
	if streamers.Workspace != nil {
		registry.Register(mutation.EntityWorkspace, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			streamers.Workspace.Publish(rworkspace.WorkspaceTopic{WorkspaceID: evt.WorkspaceID}, rworkspace.WorkspaceEvent{
				Type: "delete",
				Workspace: &apiv1.Workspace{
					WorkspaceId: evt.ID.Bytes(),
				},
			})
		})
	}

	// Environment entity
	if streamers.Environment != nil {
		registry.Register(mutation.EntityEnvironment, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			streamers.Environment.Publish(renv.EnvironmentTopic{WorkspaceID: evt.WorkspaceID}, renv.EnvironmentEvent{
				Type: "delete",
				Environment: &envapiv1.Environment{
					EnvironmentId: evt.ID.Bytes(),
					WorkspaceId:   evt.WorkspaceID.Bytes(),
				},
			})
		})
	}

	// Environment Variable entity
	if streamers.EnvironmentVariable != nil {
		registry.Register(mutation.EntityEnvironmentValue, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			streamers.EnvironmentVariable.Publish(renv.EnvironmentVariableTopic{WorkspaceID: evt.WorkspaceID, EnvironmentID: evt.ParentID}, renv.EnvironmentVariableEvent{
				Type: "delete",
				Variable: &envapiv1.EnvironmentVariable{
					EnvironmentVariableId: evt.ID.Bytes(),
					EnvironmentId:         evt.ParentID.Bytes(),
				},
			})
		})
	}

	// File entity
	if streamers.File != nil {
		registry.Register(mutation.EntityFile, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			streamers.File.Publish(rfile.FileTopic{WorkspaceID: evt.WorkspaceID}, rfile.FileEvent{
				Type: "delete",
				File: &filesystemv1.File{
					FileId:      evt.ID.Bytes(),
					WorkspaceId: evt.WorkspaceID.Bytes(),
				},
			})
		})
	}

	// HTTP entity
	if httpStreamers.Http != nil {
		registry.Register(mutation.EntityHTTP, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			httpStreamers.Http.Publish(rhttp.HttpTopic{WorkspaceID: evt.WorkspaceID}, rhttp.HttpEvent{
				Type:    "delete",
				IsDelta: evt.IsDelta,
				Http:    &httpv1.Http{HttpId: evt.ID.Bytes()},
			})
		})
	}

	// HTTP Header entity
	if httpStreamers.HttpHeader != nil {
		registry.Register(mutation.EntityHTTPHeader, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			httpStreamers.HttpHeader.Publish(rhttp.HttpHeaderTopic{WorkspaceID: evt.WorkspaceID}, rhttp.HttpHeaderEvent{
				Type:       "delete",
				IsDelta:    evt.IsDelta,
				HttpHeader: &httpv1.HttpHeader{HttpHeaderId: evt.ID.Bytes()},
			})
		})
	}

	// HTTP Search Param entity
	if httpStreamers.HttpSearchParam != nil {
		registry.Register(mutation.EntityHTTPParam, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			httpStreamers.HttpSearchParam.Publish(rhttp.HttpSearchParamTopic{WorkspaceID: evt.WorkspaceID}, rhttp.HttpSearchParamEvent{
				Type:            "delete",
				IsDelta:         evt.IsDelta,
				HttpSearchParam: &httpv1.HttpSearchParam{HttpSearchParamId: evt.ID.Bytes()},
			})
		})
	}

	// HTTP Body Form entity
	if httpStreamers.HttpBodyForm != nil {
		registry.Register(mutation.EntityHTTPBodyForm, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			httpStreamers.HttpBodyForm.Publish(rhttp.HttpBodyFormTopic{WorkspaceID: evt.WorkspaceID}, rhttp.HttpBodyFormEvent{
				Type:         "delete",
				IsDelta:      evt.IsDelta,
				HttpBodyForm: &httpv1.HttpBodyFormData{HttpBodyFormDataId: evt.ID.Bytes()},
			})
		})
	}

	// HTTP Body URL Encoded entity
	if httpStreamers.HttpBodyUrlEncoded != nil {
		registry.Register(mutation.EntityHTTPBodyURL, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			httpStreamers.HttpBodyUrlEncoded.Publish(rhttp.HttpBodyUrlEncodedTopic{WorkspaceID: evt.WorkspaceID}, rhttp.HttpBodyUrlEncodedEvent{
				Type:               "delete",
				IsDelta:            evt.IsDelta,
				HttpBodyUrlEncoded: &httpv1.HttpBodyUrlEncoded{HttpBodyUrlEncodedId: evt.ID.Bytes()},
			})
		})
	}

	// HTTP Body Raw entity
	if httpStreamers.HttpBodyRaw != nil {
		registry.Register(mutation.EntityHTTPBodyRaw, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			httpStreamers.HttpBodyRaw.Publish(rhttp.HttpBodyRawTopic{WorkspaceID: evt.WorkspaceID}, rhttp.HttpBodyRawEvent{
				Type:        "delete",
				IsDelta:     evt.IsDelta,
				HttpBodyRaw: &httpv1.HttpBodyRaw{HttpId: evt.ParentID.Bytes()},
			})
		})
	}

	// HTTP Assert entity
	if httpStreamers.HttpAssert != nil {
		registry.Register(mutation.EntityHTTPAssert, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			httpStreamers.HttpAssert.Publish(rhttp.HttpAssertTopic{WorkspaceID: evt.WorkspaceID}, rhttp.HttpAssertEvent{
				Type:       "delete",
				IsDelta:    evt.IsDelta,
				HttpAssert: &httpv1.HttpAssert{HttpAssertId: evt.ID.Bytes()},
			})
		})
	}

	// Flow entity
	if streamers.Flow != nil {
		registry.Register(mutation.EntityFlow, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			streamers.Flow.Publish(rflowv2.FlowTopic{WorkspaceID: evt.WorkspaceID}, rflowv2.FlowEvent{
				Type: "delete",
				Flow: &flowv1.Flow{FlowId: evt.ID.Bytes()},
			})
		})
	}

	// Flow Node entity
	if streamers.Node != nil {
		registry.Register(mutation.EntityFlowNode, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			streamers.Node.Publish(rflowv2.NodeTopic{FlowID: evt.ParentID}, rflowv2.NodeEvent{
				Type:   "delete",
				FlowID: evt.ParentID,
				Node:   &flowv1.Node{NodeId: evt.ID.Bytes()},
			})
		})
	}

	// Flow Edge entity
	if streamers.Edge != nil {
		registry.Register(mutation.EntityFlowEdge, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			streamers.Edge.Publish(rflowv2.EdgeTopic{FlowID: evt.ParentID}, rflowv2.EdgeEvent{
				Type:   "delete",
				FlowID: evt.ParentID,
				Edge:   &flowv1.Edge{EdgeId: evt.ID.Bytes()},
			})
		})
	}

	// Flow Variable entity
	if streamers.FlowVariable != nil {
		registry.Register(mutation.EntityFlowVariable, func(evt mutation.Event) {
			if evt.Op != mutation.OpDelete {
				return
			}
			streamers.FlowVariable.Publish(rflowv2.FlowVariableTopic{FlowID: evt.ParentID}, rflowv2.FlowVariableEvent{
				Type:     "delete",
				FlowID:   evt.ParentID,
				Variable: mflow.FlowVariable{ID: evt.ID},
			})
		})
	}
}
