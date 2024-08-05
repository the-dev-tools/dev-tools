package main

import (
	"devtools-backend/internal/api"
	"devtools-backend/internal/api/auth"
	"devtools-backend/internal/api/collection"
	"devtools-backend/internal/api/flow"
	"devtools-backend/internal/api/node"
	"devtools-backend/pkg/db/turso"
	"devtools-backend/pkg/service/scollection"
	"devtools-backend/pkg/service/scollection/sitemapi"
	"devtools-backend/pkg/service/scollection/sitemfolder"
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

	// Environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		log.Fatal(errors.New("DB_NAME env var is required"))
	}

	dbToken := os.Getenv("DB_TOKEN")
	if dbToken == "" {
		log.Fatal(errors.New("DB_TOKEN env var is required"))
	}

	dbUsername := os.Getenv("DB_USERNAME")
	if dbUsername == "" {
		log.Fatal(errors.New("DB_USERNAME env var is required"))
	}

	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		log.Fatal(errors.New("HMAC_SECRET env var is required"))
	}
	hmacSecretBytes := []byte(hmacSecret)

	db, err := turso.NewTurso(dbName, dbUsername, dbToken)
	if err != nil {
		log.Fatal(err)
	}

	// Tables
	err = scollection.PrepareTables(db)
	if err != nil {
		log.Fatal(err)
	}

	sitemapi.PrepareTables(db)
	if err != nil {
		log.Fatal(err)
	}

	sitemfolder.PrepareTables(db)
	if err != nil {
		log.Fatal(err)
	}

	// Prepared statements
	err = scollection.PrepareStatements(db)
	if err != nil {
		log.Fatal(err)
	}

	err = sitemapi.PrepareStatements(db)
	if err != nil {
		log.Fatal(err)
	}

	err = sitemfolder.PrepareStatements(db)
	if err != nil {
		log.Fatal(err)
	}

	// Services Connect RPC
	var services []api.Service
	authService, err := auth.CreateService(hmacSecretBytes)
	if err != nil {
		log.Fatal(err)
	}
	services = append(services, *authService)

	client := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	defer client.Close()

	nodeService, err := node.CreateService(client)
	if err != nil {
		log.Fatal(err)
	}
	services = append(services, *nodeService)

	flowService, err := flow.CreateService(hmacSecretBytes)
	if err != nil {
		log.Fatal(err)
	}
	services = append(services, *flowService)

	collectionService, err := collection.CreateService(db)
	if err != nil {
		log.Fatal(err)
	}
	services = append(services, *collectionService)

	// Start services
	go func() {
		err := api.ListenServices(services, port)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for signal
	<-sc
}
