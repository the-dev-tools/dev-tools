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
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/tursolocal"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/middleware/mwcompress"
	"the-dev-tools/server/internal/api/rbody"
	"the-dev-tools/server/internal/api/rcollection"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/internal/api/redge"
	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/internal/api/resultapi"
	"the-dev-tools/server/internal/api/rexport"
	"the-dev-tools/server/internal/api/rflow"
	"the-dev-tools/server/internal/api/rflowvariable"
	"the-dev-tools/server/internal/api/rimport"
	"the-dev-tools/server/internal/api/ritemapi"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/internal/api/ritemfolder"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/internal/api/rnode"
	"the-dev-tools/server/internal/api/rnodeexecution"
	"the-dev-tools/server/internal/api/rreference"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/internal/api/rtag"
	"the-dev-tools/server/internal/api/rvar"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowtag"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/stag"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"

	"connectrpc.com/connect"
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

	collectionService := scollection.New(queries, logger)
	workspaceService := sworkspace.New(queries)
	workspaceUserService := sworkspacesusers.New(queries)
	userService := suser.New(queries)
	endpointService := sitemapi.New(queries)
	folderService := sitemfolder.New(queries)
	exampleService := sitemapiexample.New(queries)
	exampleHeaderService := sexampleheader.New(queries)
	exampleQueryService := sexamplequery.New(queries)
	bodyRawService := sbodyraw.New(queries)
	bodyFormService := sbodyform.New(queries)
	bodyUrlService := sbodyurl.New(queries)
	exampleResponseService := sexampleresp.New(queries)
	exampleResponseHeaderService := sexamplerespheader.New(queries)
	assertService := sassert.New(queries)
	assertResultService := sassertres.New(queries)
	variableService := svar.New(queries)
	environmentService := senv.New(queries)
	tagService := stag.New(queries)

	// Flow
	flowService := sflow.New(queries)
	flowTagService := sflowtag.New(queries)
	flowEdgeService := sedge.New(queries)
	flowVariableService := sflowvariable.New(queries)

	// nodes
	flowNodeService := snode.New(queries)
	flowNodeRequestSevice := snoderequest.New(queries)
	flowNodeForService := snodefor.New(queries)
	flowNodeForeachService := snodeforeach.New(queries)
	flowNodeCondition := snodeif.New(queries)
	flowNodeNoOpService := snodenoop.New(queries)
	flowNodeJsService := snodejs.New(queries)
	nodeExecutionService := snodeexecution.New(queries)

	// log/console
	logMap := logconsole.NewLogChanMap()

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

	workspaceSrv := rworkspace.New(currentDB, workspaceService, workspaceUserService, userService, environmentService)
	newServiceManager.AddService(rworkspace.CreateService(workspaceSrv, opitonsAll))

	// Collection Service
	collectionSrv := rcollection.New(currentDB, collectionService, workspaceService,
		userService)
	newServiceManager.AddService(rcollection.CreateService(collectionSrv, opitonsAll))

	// Collection Item Service
	collectionItemSrv := rcollectionitem.New(currentDB, collectionService, userService, folderService, endpointService, exampleService, exampleResponseService)
	newServiceManager.AddService(rcollectionitem.CreateService(collectionItemSrv, opitonsAll))

	// Result API Service
	resultapiSrv := resultapi.New(currentDB, userService, collectionService, endpointService, exampleService, workspaceService, exampleResponseService, exampleResponseHeaderService, assertService, assertResultService)
	newServiceManager.AddService(resultapi.CreateService(resultapiSrv, opitonsAll))

	// Item API Service
	itemapiSrv := ritemapi.New(currentDB, endpointService, collectionService,
		folderService, userService, exampleService, exampleResponseService)
	newServiceManager.AddService(ritemapi.CreateService(itemapiSrv, opitonsAll))

	// Folder API Service
	folderItemSrv := ritemfolder.New(currentDB, folderService, userService, collectionService)
	newServiceManager.AddService(ritemfolder.CreateService(folderItemSrv, opitonsAll))

	// Api Item Example
	itemApiExampleSrv := ritemapiexample.New(currentDB, exampleService, endpointService, folderService,
		workspaceService, collectionService, userService, exampleHeaderService, exampleQueryService, bodyFormService, bodyUrlService,
		bodyRawService, exampleResponseHeaderService, exampleResponseService, environmentService, variableService, assertService, assertResultService, logMap)
	newServiceManager.AddService(ritemapiexample.CreateService(itemApiExampleSrv, opitonsAll))

	requestSrv := rrequest.New(currentDB, collectionService, userService, endpointService, exampleService, exampleHeaderService, exampleQueryService, assertService)
	newServiceManager.AddService(rrequest.CreateService(requestSrv, opitonsAll))

	// BodyRaw Service
	bodySrv := rbody.New(currentDB, collectionService, exampleService, userService, bodyFormService, bodyUrlService, bodyRawService)
	newServiceManager.AddService(rbody.CreateService(bodySrv, opitonsAll))

	// Env Service
	envSrv := renv.New(currentDB, environmentService, variableService, userService)
	newServiceManager.AddService(renv.CreateService(envSrv, opitonsAll))

	// Var Service
	varSrv := rvar.New(currentDB, userService, environmentService, variableService)
	newServiceManager.AddService(rvar.CreateService(varSrv, opitonsAll))

	tagSrv := rtag.New(currentDB, workspaceService, userService, tagService)
	newServiceManager.AddService(rtag.CreateService(tagSrv, opitonsAll))

	// Flow Service
	flowSrv := rflow.New(currentDB, workspaceService, userService, tagService,
		// flow
		flowService, flowTagService, flowEdgeService, flowVariableService,
		// req
		endpointService, exampleService, exampleQueryService, exampleHeaderService,
		// body
		bodyRawService, bodyFormService, bodyUrlService,
		// resp
		exampleResponseService, exampleResponseHeaderService, assertService, assertResultService,
		// subnodes
		flowNodeService, flowNodeRequestSevice, flowNodeForService, flowNodeForeachService,
		flowNodeNoOpService, *flowNodeCondition, flowNodeJsService, nodeExecutionService, logMap)
	newServiceManager.AddService(rflow.CreateService(flowSrv, opitonsAll))

	// Node Service
	nodeSrv := rnode.NewNodeServiceRPC(currentDB, userService,
		flowService, *flowNodeCondition,
		flowNodeRequestSevice, flowNodeForService, flowNodeForeachService, flowNodeService, flowNodeNoOpService, flowNodeJsService,
		endpointService, exampleService, exampleQueryService, exampleHeaderService, bodyRawService, bodyFormService, bodyUrlService,
		nodeExecutionService)
	newServiceManager.AddService(rnode.CreateService(nodeSrv, opitonsAll))

	// NodeExecution Service
	nodeExecutionSrv := rnodeexecution.New(&nodeExecutionService, &flowNodeService, &flowService, &userService)
	nodeExecutionService_svc, err := rnodeexecution.CreateService(nodeExecutionSrv, opitonsAll)
	if err != nil {
		log.Fatal(err)
	}
	newServiceManager.AddService(nodeExecutionService_svc, err)

	// Edge Service
	edgeSrv := redge.NewEdgeServiceRPC(currentDB, flowService, userService, flowEdgeService, flowNodeService)
	newServiceManager.AddService(redge.CreateService(edgeSrv, opitonsAll))

	// Log Service
	logSrv := rlog.NewRlogRPC(logMap)
	newServiceManager.AddService(rlog.CreateService(logSrv, opitonsAll))

	// Refernce Service
	refServiceRPC := rreference.NewNodeServiceRPC(currentDB, userService, workspaceService, environmentService, variableService, exampleResponseService, exampleResponseHeaderService,
		flowService, flowNodeService, flowNodeRequestSevice, flowVariableService, flowEdgeService, nodeExecutionService)
	newServiceManager.AddService(rreference.CreateService(refServiceRPC, opitonsAll))

	importServiceRPC := rimport.New(currentDB, workspaceService, collectionService, userService, folderService, endpointService, exampleService, exampleResponseService, assertService)
	importService, err := rimport.CreateService(importServiceRPC, opitonsAll)
	if err != nil {
		log.Fatal(err)
	}
	newServiceManager.AddService(importService, err)

	flowServiceRPC := rflowvariable.New(currentDB, flowService, userService, flowVariableService)
	newServiceManager.AddService(rflowvariable.CreateService(flowServiceRPC, opitonsAll))

	exportServiceRPC := rexport.New(
		currentDB,
		workspaceService, collectionService, folderService,
		endpointService, exampleService, exampleHeaderService, exampleQueryService, assertService,
		bodyRawService, bodyFormService, bodyUrlService,
		exampleResponseService, exampleResponseHeaderService, assertResultService,
		// flow
		flowService,
		// nodes
		flowNodeService, flowEdgeService, flowVariableService, flowNodeRequestSevice,
		*flowNodeCondition, flowNodeNoOpService,
		flowNodeForService, flowNodeForeachService, flowNodeJsService,
		environmentService, variableService,
	)
	exportService, err := rexport.CreateService(exportServiceRPC, opitonsAll)
	if err != nil {
		log.Fatal(err)
	}
	newServiceManager.AddService(exportService, err)

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
	db, a, err := tursolocal.NewTursoLocal(ctx, dbName, dbPath, encryptKey)
	if err != nil {
		return nil, nil, err
	}
	return db, a, nil
}
