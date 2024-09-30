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
	"dev-tools-db/pkg/tursoembedded"
	"errors"
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

	client := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	defer client.Close()

	dbEmbedded, dbCloseFunc, err := GetDBEmbedded()
	if err != nil {
		log.Fatal(err)
	}
	defer dbCloseFunc()

	// Services Connect RPC
	newServiceManager := NewServiceManager(10)
	newServiceManager.AddService(auth.CreateService(ctx, dbEmbedded, hmacSecretBytes))
	newServiceManager.AddService(collection.CreateService(ctx, dbEmbedded, hmacSecretBytes))
	newServiceManager.AddService(node.CreateService(client))
	newServiceManager.AddService(resultapi.CreateService(ctx, dbEmbedded, hmacSecretBytes))
	newServiceManager.AddService(rworkspace.CreateService(ctx, hmacSecretBytes, dbEmbedded))
	newServiceManager.AddService(ritemapi.CreateService(ctx, dbEmbedded, hmacSecretBytes))
	newServiceManager.AddService(ritemfolder.CreateService(ctx, dbEmbedded, hmacSecretBytes))
	newServiceManager.AddService(ritemapiexample.CreateService(ctx, dbEmbedded, hmacSecretBytes))
	newServiceManager.AddService(rbody.CreateService(ctx, dbEmbedded, hmacSecretBytes))
	newServiceManager.AddService(renv.CreateService(ctx, dbEmbedded, hmacSecretBytes))
	newServiceManager.AddService(rvar.CreateService(ctx, dbEmbedded, hmacSecretBytes))

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

/*
func GetDB() (*sql.DB, error) {
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		return nil, errors.New("DB_NAME env var is required")
	}

	dbToken := os.Getenv("DB_TOKEN")
	if dbToken == "" {
		return nil, errors.New("DB_TOKEN env var is required")
	}

	dbUsername := os.Getenv("DB_USERNAME")
	if dbUsername == "" {
		return nil, errors.New("DB_USERNAME env var is required")
	}

	db, err := tursoclient.NewTurso(dbName, dbUsername, dbToken)
	if err != nil {
		return nil, err
	}
	return db, nil
}
*/

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
