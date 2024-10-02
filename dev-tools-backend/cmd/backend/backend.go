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
	devtoolsdb "dev-tools-db"
	"dev-tools-db/pkg/tursoembedded"
	"dev-tools-db/pkg/tursolocal"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bufbuild/httplb"
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

	client := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	defer client.Close()

	// Services Connect RPC
	newServiceManager := NewServiceManager(15)
	newServiceManager.AddService(auth.CreateService(ctx, currentDB, hmacSecretBytes))
	newServiceManager.AddService(collection.CreateService(ctx, currentDB, hmacSecretBytes))
	newServiceManager.AddService(node.CreateService(client))
	newServiceManager.AddService(resultapi.CreateService(ctx, currentDB, hmacSecretBytes))
	newServiceManager.AddService(rworkspace.CreateService(ctx, hmacSecretBytes, currentDB))
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
	sm.s = append(sm.s, *s)
	if e != nil {
		log.Fatal(e)
	}
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
