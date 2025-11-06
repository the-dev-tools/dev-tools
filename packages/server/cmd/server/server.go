package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
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
	// "the-dev-tools/server/internal/api/redge"
	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/internal/api/rflowv2"

	// "the-dev-tools/server/internal/api/rflowvariable"
	// "the-dev-tools/server/internal/api/rhealth"
	"the-dev-tools/server/internal/api/rhttp"

	// "the-dev-tools/server/internal/api/rlog"
	// "the-dev-tools/server/internal/api/rnode"
	// "the-dev-tools/server/internal/api/rnodeexecution"
	// "the-dev-tools/server/internal/api/rreference"

	// "the-dev-tools/server/internal/api/rvar"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/eventstream/memory"
	// "the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/service/flow/sedge"
	// "the-dev-tools/server/pkg/service/sassert"
	// "the-dev-tools/server/pkg/service/sassertres"
	// "the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	// "the-dev-tools/server/pkg/service/sbodyurl"

	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	// "the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	// "the-dev-tools/server/pkg/service/sflowtag"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpassert"
	"the-dev-tools/server/pkg/service/shttpbodyform"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"
	"the-dev-tools/server/pkg/service/shttpheader"
	"the-dev-tools/server/pkg/service/shttpsearchparam"
	// "the-dev-tools/server/pkg/service/sitemapi"
	// "the-dev-tools/server/pkg/service/sitemapiexample"

	"the-dev-tools/server/pkg/service/snode"
	// "the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	// "the-dev-tools/server/pkg/service/stag"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
)

func main() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

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

	logger := slog.New(loggerHandler)

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
		log.Fatal(errors.New("HMAC_SECRET env var is required"))
	}

	dbMode := os.Getenv("DB_MODE")
	if dbMode == "" {
		log.Fatal(errors.New("DB_MODE env var is required"))
	}
	fmt.Println("DB_MODE: ", dbMode)

	var currentDB *sql.DB
	var dbCloseFunc func()
	var err error
	switch dbMode {
	case devtoolsdb.LOCAL:
		currentDB, dbCloseFunc, err = GetDBLocal(ctx)
	default:
		err = errors.New("invalid db mode")
	}
	if err != nil {
		log.Fatal(err)
	}
	defer dbCloseFunc()

	queries, err := gen.Prepare(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	workspaceService := sworkspace.New(queries)
	workspaceUserService := sworkspacesusers.New(queries)
	userService := suser.New(queries)
	// endpointService := sitemapi.New(queries)

	// exampleService := sitemapiexample.New(queries)
	exampleHeaderService := sexampleheader.New(queries)
	exampleQueryService := sexamplequery.New(queries)
	bodyRawService := sbodyraw.New(queries)
	// bodyFormService := sbodyform.New(queries)
	// bodyUrlService := sbodyurl.New(queries)
	exampleResponseService := sexampleresp.New(queries)
	// exampleResponseHeaderService := sexamplerespheader.New(queries)
	// assertService := sassert.New(queries)
	// assertResultService := sassertres.New(queries)
	variableService := svar.New(queries, logger)
	environmentService := senv.New(queries, logger)
	// tagService := stag.New(queries)
	httpService := shttp.New(queries, logger)

	// HTTP child entity services
	httpHeaderService := shttpheader.New(queries)
	httpSearchParamService := shttpsearchparam.New(queries)
	httpBodyFormService := shttpbodyform.New(queries)
	httpBodyUrlEncodedService := shttpbodyurlencoded.New(queries)
	httpAssertService := shttpassert.New(queries)
	// Aggregated HTTP services used by flow execution
	httpHeaderAgg := shttp.NewHttpHeaderService(queries)
	httpSearchAgg := shttp.NewHttpSearchParamService(queries)
	httpBodyFormAgg := shttp.NewHttpBodyFormService(queries)
	httpBodyUrlAgg := shttp.NewHttpBodyUrlencodedService(queries)
	httpAssertAgg := shttp.NewHttpAssertService(queries)

	// Flow
	flowService := sflow.New(queries)
	// flowTagService := sflowtag.New(queries)
	flowEdgeService := sedge.New(queries)
	flowVariableService := sflowvariable.New(queries)

	// nodes
	flowNodeService := snode.New(queries)
	flowNodeRequestSevice := snoderequest.New(queries)
	flowNodeForService := snodefor.New(queries)
	flowNodeForeachService := snodeforeach.New(queries)
	flowNodeConditionService := snodeif.New(queries)
	flowNodeNoOpService := snodenoop.New(queries)
	flowNodeJsService := snodejs.New(queries)
	// nodeExecutionService := snodeexecution.New(queries)

	// log/console
	// logMap := logconsole.NewLogChanMap()

	var optionsCompress, optionsAuth, opitonsAll []connect.HandlerOption
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
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}

	optionsAuth = append(optionsCompress, connect.WithInterceptors(mwauth.NewAuthInterceptor()))
	opitonsAll = append(optionsAuth, optionsCompress...)

	// Services Connect RPC
	newServiceManager := NewServiceManager(30)

	// healthSrv := rhealth.New()
	// newServiceManager.AddService(rhealth.CreateService(healthSrv, optionsCompress))

	workspaceStreamer := memory.NewInMemorySyncStreamer[rworkspace.WorkspaceTopic, rworkspace.WorkspaceEvent]()
	defer workspaceStreamer.Shutdown()

	workspaceSrv := rworkspace.New(currentDB, workspaceService, workspaceUserService, userService, environmentService, workspaceStreamer)
	newServiceManager.AddService(rworkspace.CreateService(workspaceSrv, opitonsAll))

	// Env Service
	environmentStreamer := memory.NewInMemorySyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent]()
	defer environmentStreamer.Shutdown()
	environmentVariableStreamer := memory.NewInMemorySyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent]()
	defer environmentVariableStreamer.Shutdown()

	envSrv := renv.New(currentDB, environmentService, variableService, userService, workspaceService, environmentStreamer, environmentVariableStreamer)
	newServiceManager.AddService(renv.CreateService(envSrv, opitonsAll))

	// HTTP Service
	httpStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]()
	defer httpStreamer.Shutdown()

	// HTTP child entity streamers
	httpHeaderStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]()
	defer httpHeaderStreamer.Shutdown()
	httpSearchParamStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]()
	defer httpSearchParamStreamer.Shutdown()
	httpBodyFormStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]()
	defer httpBodyFormStreamer.Shutdown()
	httpBodyUrlEncodedStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]()
	defer httpBodyUrlEncodedStreamer.Shutdown()
	httpAssertStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent]()
	defer httpAssertStreamer.Shutdown()
	httpVersionStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpVersionTopic, rhttp.HttpVersionEvent]()
	defer httpVersionStreamer.Shutdown()
	httpResponseStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent]()
	defer httpResponseStreamer.Shutdown()
	httpResponseHeaderStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent]()
	defer httpResponseHeaderStreamer.Shutdown()
	httpResponseAssertStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent]()
	defer httpResponseAssertStreamer.Shutdown()
	httpBodyRawStreamer := memory.NewInMemorySyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]()
	defer httpBodyRawStreamer.Shutdown()

	httpSrv := rhttp.New(currentDB, httpService, userService, workspaceService, workspaceUserService, environmentService, variableService, exampleHeaderService, exampleQueryService, bodyRawService, exampleResponseService, httpHeaderService, httpSearchParamService, httpBodyFormService, httpBodyUrlEncodedService, httpAssertService, httpStreamer, httpHeaderStreamer, httpSearchParamStreamer, httpBodyFormStreamer, httpBodyUrlEncodedStreamer, httpAssertStreamer, httpVersionStreamer, httpResponseStreamer, httpResponseHeaderStreamer, httpResponseAssertStreamer, httpBodyRawStreamer)
	newServiceManager.AddService(rhttp.CreateService(httpSrv, opitonsAll))

	nodeStreamer := memory.NewInMemorySyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent]()
	defer nodeStreamer.Shutdown()
	edgeStreamer := memory.NewInMemorySyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent]()
	defer edgeStreamer.Shutdown()
	flowVariableStreamer := memory.NewInMemorySyncStreamer[rflowv2.FlowVariableTopic, rflowv2.FlowVariableEvent]()
	defer flowVariableStreamer.Shutdown()
	flowVersionStreamer := memory.NewInMemorySyncStreamer[rflowv2.FlowVersionTopic, rflowv2.FlowVersionEvent]()
	defer flowVersionStreamer.Shutdown()
	noopStreamer := memory.NewInMemorySyncStreamer[rflowv2.NoOpTopic, rflowv2.NoOpEvent]()
	defer noopStreamer.Shutdown()
	forStreamer := memory.NewInMemorySyncStreamer[rflowv2.ForTopic, rflowv2.ForEvent]()
	defer forStreamer.Shutdown()
	conditionStreamer := memory.NewInMemorySyncStreamer[rflowv2.ConditionTopic, rflowv2.ConditionEvent]()
	defer conditionStreamer.Shutdown()

	flowSrvV2 := rflowv2.New(
		&workspaceService,
		&flowService,
		&flowEdgeService,
		&flowNodeService,
		&flowNodeRequestSevice,
		&flowNodeForService,
		&flowNodeForeachService,
		flowNodeConditionService,
		&flowNodeNoOpService,
		&flowNodeJsService,
		&flowVariableService,
		&httpService,
		httpHeaderAgg,
		httpSearchAgg,
		&httpBodyFormAgg,
		httpBodyUrlAgg,
		&httpAssertAgg,
		nodeStreamer,
		edgeStreamer,
		flowVariableStreamer,
		flowVersionStreamer,
		noopStreamer,
		forStreamer,
		conditionStreamer,
	)
	newServiceManager.AddService(rflowv2.CreateService(flowSrvV2, opitonsAll))

	// Var Service
	// varSrv := rvar.New(currentDB, userService, environmentService, variableService)
	// newServiceManager.AddService(rvar.CreateService(varSrv, opitonsAll))

	// Flow Service
	// flowSrv := rflow.New(currentDB, workspaceService, userService, tagService,
	// 	// flow
	// 	flowService, flowTagService, flowEdgeService, flowVariableService, environmentService, variableService,
	// 	// req
	// 	endpointService, exampleService, exampleQueryService, exampleHeaderService,
	// 	// body
	// 	bodyRawService, bodyFormService, bodyUrlService,
	// 	// resp
	// 	exampleResponseService, exampleResponseHeaderService, assertService, assertResultService,
	// 	// subnodes
	// 	flowNodeService, flowNodeRequestSevice, flowNodeForService, flowNodeForeachService,
	// 	flowNodeNoOpService, *flowNodeCondition, flowNodeJsService, nodeExecutionService, logMap, logger)
	// newServiceManager.AddService(rflow.CreateService(flowSrv, opitonsAll))

	// Node Service
	// nodeSrv := rnode.NewNodeServiceRPC(currentDB, userService,
	// 	flowService, *flowNodeCondition,
	// 	flowNodeRequestSevice, flowNodeForService, flowNodeForeachService, flowNodeService, flowNodeNoOpService, flowNodeJsService,
	// 	endpointService, exampleService, exampleQueryService, exampleHeaderService, bodyRawService, bodyFormService, bodyUrlService,
	// 	nodeExecutionService)
	// newServiceManager.AddService(rnode.CreateService(nodeSrv, opitonsAll))

	// NodeExecution Service
	// nodeExecutionSrv := rnodeexecution.New(&nodeExecutionService, &flowNodeService, &flowService, &userService, &exampleResponseService, &flowNodeRequestSevice)
	// nodeExecutionService_svc, err := rnodeexecution.CreateService(nodeExecutionSrv, opitonsAll)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// newServiceManager.AddService(nodeExecutionService_svc, err)

	// Edge Service
	// edgeSrv := redge.NewEdgeServiceRPC(currentDB, flowService, userService, flowEdgeService, flowNodeService)
	// newServiceManager.AddService(redge.CreateService(edgeSrv, opitonsAll))

	// Log Service
	// logSrv := rlog.NewRlogRPC(logMap)
	// newServiceManager.AddService(rlog.CreateService(logSrv, opitonsAll))

	// Refernce Service
	// refServiceRPC := rreference.NewNodeServiceRPC(currentDB, userService, workspaceService, environmentService, variableService, exampleResponseService, exampleResponseHeaderService,
	// 	flowService, flowNodeService, flowNodeRequestSevice, flowVariableService, flowEdgeService, nodeExecutionService)
	// newServiceManager.AddService(rreference.CreateService(refServiceRPC, opitonsAll))

	// flowServiceRPC := rflowvariable.New(currentDB, flowService, userService, flowVariableService)
	// newServiceManager.AddService(rflowvariable.CreateService(flowServiceRPC, opitonsAll))

	// Start services
	go func() {
		err := api.ListenServices(newServiceManager.GetServices(), port)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for signal
	<-sc
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
