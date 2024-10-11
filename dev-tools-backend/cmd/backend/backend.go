package main

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/auth"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/node"
	"dev-tools-backend/internal/api/rbody"
	"dev-tools-backend/internal/api/renv"
	"dev-tools-backend/internal/api/resultapi"
	"dev-tools-backend/internal/api/ritemapi"
	"dev-tools-backend/internal/api/ritemapiexample"
	"dev-tools-backend/internal/api/ritemfolder"
	"dev-tools-backend/internal/api/rvar"
	"dev-tools-backend/internal/api/rworkspace"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/senv"
	"dev-tools-backend/pkg/service/sexampleheader"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/sitemfolder"
	"dev-tools-backend/pkg/service/sresultapi"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/service/sworkspacesusers"
	devtoolsdb "dev-tools-db"
	"dev-tools-db/pkg/tursoembedded"
	"dev-tools-db/pkg/tursolocal"
	"dev-tools-mail/pkg/emailclient"
	"dev-tools-mail/pkg/emailinvite"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bufbuild/httplb"
	"github.com/magiclabs/magic-admin-go"

	magiccl "github.com/magiclabs/magic-admin-go/client"
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
	hmacSecretBytes := []byte(hmacSecret)

	dbMode := os.Getenv("DB_MODE")
	if dbMode == "" {
		log.Fatal(errors.New("DB_MODE env var is required"))
	}
	fmt.Println("DB_MODE: ", dbMode)

	var currentDB *sql.DB
	var dbCloseFunc func()
	var err error
	switch dbMode {
	case devtoolsdb.EMBEDDED:
		currentDB, dbCloseFunc, err = GetDBEmbedded()
	case devtoolsdb.LOCAL:
		currentDB, dbCloseFunc, err = GetDBLocal()
	case devtoolsdb.REMOTE:
		err = errors.New("remote db mode is not supported")
	default:
		err = errors.New("invalid db mode")
	}
	if err != nil {
		log.Fatal(err)
	}
	defer dbCloseFunc()

	clientHttp := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	defer clientHttp.Close()

	cs, err := scollection.New(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	ws, err := sworkspace.New(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	wus, err := sworkspacesusers.New(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	us, err := suser.New(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	ias, err := sitemapi.New(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	ifs, err := sitemfolder.New(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	ras, err := sresultapi.New(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	iaes, err := sitemapiexample.New(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	hes, err := sexampleheader.New(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	es, err := senv.New(ctx, currentDB)
	if err != nil {
		log.Fatal(err)
	}

	magicLinkSecret := os.Getenv("MAGIC_LINK_SECRET")
	if magicLinkSecret == "" {
		log.Fatal("MAGIC_LINK_SECRET env var is required")
	}

	cl := magic.NewClientWithRetry(5, time.Second, 10*time.Second)
	MagicLinkClient, err := magiccl.New(magicLinkSecret, cl)
	if err != nil {
		log.Fatal(err)
	}

	AWS_ACCESS_KEY := os.Getenv("AWS_ACCESS_KEY")
	if AWS_ACCESS_KEY == "" {
		log.Fatalf("AWS_ACCESS_KEY is empty")
	}
	AWS_SECRET_KEY := os.Getenv("AWS_SECRET_KEY")
	if AWS_SECRET_KEY == "" {
		log.Fatalf("AWS_SECRET_KEY is empty")
	}

	emailClient, err := emailclient.NewClient(AWS_ACCESS_KEY, AWS_SECRET_KEY, "")
	if err != nil {
		log.Fatalf("failed to create email client: %v", err)
	}

	path := os.Getenv("EMAIL_INVITE_TEMPLATE_PATH")
	if path == "" {
		log.Fatalf("EMAIL_INVITE_TEMPLATE_PATH is empty")
	}
	emailInviteManager, err := emailinvite.NewEmailTemplateFile(path, emailClient)
	if err != nil {
		log.Fatalf("failed to create email invite manager: %v", err)
	}

	// Services Connect RPC
	newServiceManager := NewServiceManager(15)
	authSrv := auth.New(*MagicLinkClient, *us, *ws, *wus, hmacSecretBytes)
	newServiceManager.AddService(auth.CreateService(ctx, authSrv))
	collectionSrv := collection.New(currentDB, *cs, *ws,
		*us, *ias, *ifs, *ras, *iaes, *hes, hmacSecretBytes)
	newServiceManager.AddService(collection.CreateService(ctx, collectionSrv))

	newServiceManager.AddService(node.CreateService(clientHttp))
	resultapiSrv := resultapi.New(currentDB, *cs, *ias, *ws, *ras, hmacSecretBytes)
	newServiceManager.AddService(resultapi.CreateService(ctx, resultapiSrv))

	workspaceSrv := rworkspace.New(currentDB, *ws, *wus, *us, es, *emailClient, emailInviteManager, hmacSecretBytes)
	newServiceManager.AddService(rworkspace.CreateService(ctx, workspaceSrv))
	newServiceManager.AddService(ritemapi.CreateService(ctx, currentDB, hmacSecretBytes))
	newServiceManager.AddService(ritemfolder.CreateService(ctx, currentDB, hmacSecretBytes))
	newServiceManager.AddService(ritemapiexample.CreateService(ctx, currentDB, hmacSecretBytes))
	newServiceManager.AddService(rbody.CreateService(ctx, currentDB, hmacSecretBytes))
	newServiceManager.AddService(renv.CreateService(ctx, currentDB, hmacSecretBytes))
	newServiceManager.AddService(rvar.CreateService(ctx, currentDB, hmacSecretBytes))

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

func GetDBEmbedded() (*sql.DB, func(), error) {
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		return nil, nil, errors.New("DB_NAME env var is required")
	}
	dbToken := os.Getenv("DB_TOKEN")
	if dbToken == "" {
		return nil, nil, errors.New("DB_TOKEN env var is required")
	}
	dbUsername := os.Getenv("DB_USERNAME")
	if dbUsername == "" {
		return nil, nil, errors.New("DB_USERNAME env var is required")
	}
	dbVolumePath := os.Getenv("DB_VOLUME_PATH")
	if dbVolumePath == "" {
		return nil, nil, errors.New("DB_VOLUME_PATH env var is required")
	}

	encryptKey := os.Getenv("DB_ENCRYPTION_KEY")
	if encryptKey == "" {
		return nil, nil, errors.New("DB_ENCRYPT_KEY env var is required")
	}

	db, a, err := tursoembedded.NewTursoEmbeded(dbName, dbUsername, dbToken, dbVolumePath, encryptKey)
	if err != nil {
		return nil, nil, err
	}
	return db, a, nil
}

func GetDBLocal() (*sql.DB, func(), error) {
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
	db, a, err := tursolocal.NewTursoLocal(dbName, dbPath, encryptKey)
	if err != nil {
		return nil, nil, err
	}
	return db, a, nil
}
