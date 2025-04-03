package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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
	"the-dev-tools/server/internal/api/rimport"
	"the-dev-tools/server/internal/api/ritemapi"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/internal/api/ritemfolder"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/internal/api/rnode"
	"the-dev-tools/server/internal/api/rreference"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/internal/api/rtag"
	"the-dev-tools/server/internal/api/rvar"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sedge"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowtag"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sresultapi"
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

	cs := scollection.New(queries)
	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	ias := sitemapi.New(queries)
	ifs := sitemfolder.New(queries)
	ras := sresultapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)
	vs := svar.New(queries)
	es := senv.New(queries)
	res := sexampleresp.New(queries)
	ts := stag.New(queries)

	// Flow
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)

	// nodes
	ns := snode.New(queries)
	rns := snoderequest.New(queries)
	lfns := snodefor.New(queries)
	flens := snodeforeach.New(queries)
	ins := snodeif.New(queries)
	sns := snodenoop.New(queries)
	jsns := snodejs.New(queries)

	// log/console
	logMap := logconsole.NewLogChanMap()

	var optionsCompress, optionsAuth, opitonsAll []connect.HandlerOption
	optionsCompress = append(optionsCompress, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	optionsCompress = append(optionsCompress, connect.WithCompression("gzip", nil, nil))
	_, err = us.GetUser(ctx, mwauth.LocalDummyID)
	if err != nil {
		if errors.Is(err, suser.ErrUserNotFound) {
			defaultUser := &muser.User{
				ID: mwauth.LocalDummyID,
			}
			err = us.CreateUser(ctx, defaultUser)
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

	workspaceSrv := rworkspace.New(currentDB, ws, wus, us, es)
	newServiceManager.AddService(rworkspace.CreateService(workspaceSrv, opitonsAll))

	// Collection Service
	collectionSrv := rcollection.New(currentDB, cs, ws,
		us)
	newServiceManager.AddService(rcollection.CreateService(collectionSrv, opitonsAll))

	// Collection Item Service
	collectionItemSrv := rcollectionitem.New(currentDB, cs, us, ifs, ias, iaes, res)
	newServiceManager.AddService(rcollectionitem.CreateService(collectionItemSrv, opitonsAll))

	// Result API Service
	resultapiSrv := resultapi.New(currentDB, us, cs, ias, iaes, ws, ers, erhs, as, ars)
	newServiceManager.AddService(resultapi.CreateService(resultapiSrv, opitonsAll))

	// Item API Service
	itemapiSrv := ritemapi.New(currentDB, ias, cs,
		ifs, us, iaes, ers)
	newServiceManager.AddService(ritemapi.CreateService(itemapiSrv, opitonsAll))

	// Folder API Service
	folderItemSrv := ritemfolder.New(currentDB, ifs, us, cs)
	newServiceManager.AddService(ritemfolder.CreateService(folderItemSrv, opitonsAll))

	// Api Item Example
	itemApiExampleSrv := ritemapiexample.New(currentDB, iaes, ias, ras,
		ws, cs, us, ehs, eqs, bfs, bues,
		brs, erhs, ers, es, vs, as, ars)
	newServiceManager.AddService(ritemapiexample.CreateService(itemApiExampleSrv, opitonsAll))

	requestSrv := rrequest.New(currentDB, cs, us, iaes, ehs, eqs, as)
	newServiceManager.AddService(rrequest.CreateService(requestSrv, opitonsAll))

	// BodyRaw Service
	bodySrv := rbody.New(currentDB, cs, iaes, us, bfs, bues, brs)
	newServiceManager.AddService(rbody.CreateService(bodySrv, opitonsAll))

	// Env Service
	envSrv := renv.New(currentDB, es, vs, us)
	newServiceManager.AddService(renv.CreateService(envSrv, opitonsAll))

	// Var Service
	varSrv := rvar.New(currentDB, us, es, vs)
	newServiceManager.AddService(rvar.CreateService(varSrv, opitonsAll))

	tagSrv := rtag.New(currentDB, ws, us, ts)
	newServiceManager.AddService(rtag.CreateService(tagSrv, opitonsAll))

	// Flow Service
	flowSrv := rflow.New(currentDB, ws, us, ts,
		// flow
		fs, fts, fes,
		// req
		ias, iaes, eqs, ehs,
		// body
		brs, bfs, bues,
		// resp
		ers, erhs, as, ars,
		// subnodes
		ns, rns, lfns, flens,
		sns, *ins, jsns, logMap)
	newServiceManager.AddService(rflow.CreateService(flowSrv, opitonsAll))

	// Node Service
	nodeSrv := rnode.NewNodeServiceRPC(currentDB, us,
		fs, *ins,
		rns, lfns, flens, ns, sns, jsns,
		ias, iaes, eqs, ehs, brs, bfs, bues)
	newServiceManager.AddService(rnode.CreateService(nodeSrv, opitonsAll))

	// Edge Service
	edgeSrv := redge.NewEdgeServiceRPC(currentDB, fs, us, fes, ns)
	newServiceManager.AddService(redge.CreateService(edgeSrv, opitonsAll))

	// Log Service
	logSrv := rlog.NewRlogRPC(logMap)
	newServiceManager.AddService(rlog.CreateService(logSrv, opitonsAll))

	// Refernce Service
	refServiceRPC := rreference.NewNodeServiceRPC(currentDB, us, ws, es, vs, ers, erhs, fs, ns, rns, fes)
	newServiceManager.AddService(rreference.CreateService(refServiceRPC, opitonsAll))

	importServiceRPC := rimport.New(currentDB, ws, cs, us, ifs, ias, iaes, res)
	newServiceManager.AddService(rimport.CreateService(importServiceRPC, opitonsAll))

	exportServiceRPC := rexport.New(
		currentDB,
		ws, cs, ifs,
		ias, iaes, ehs, eqs, as,
		brs, bfs, bues,
		ers, erhs, ars,
		// flow
		fs,
		// nodes
		ns, fes, rns,
		*ins, sns,
		lfns, flens, jsns,
	)
	newServiceManager.AddService(rexport.CreateService(exportServiceRPC, opitonsAll))

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
