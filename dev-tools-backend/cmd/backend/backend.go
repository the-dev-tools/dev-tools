package main

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/auth"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/node"
	"dev-tools-backend/internal/api/rworkspace"
	"dev-tools-backend/pkg/db/turso"
	"dev-tools-backend/pkg/service/sresultapi"
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

	// PrepareTables and PrepareStatements are functions that create tables and prepared statements in the database
	err = PrepareTables(db)
	if err != nil {
		log.Fatal(err)
	}
	err = PrepareStatements(db)
	if err != nil {
		log.Fatal(err)
	}

	// Services Connect RPC
	var services []api.Service
	authService, err := auth.CreateService(db, hmacSecretBytes)
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

	/*
		flowService, err := flow.CreateService(hmacSecretBytes)
		if err != nil {
			log.Fatal(err)
		}
		services = append(services, *flowService)
	*/

	collectionService, err := collection.CreateService(ctx, db, hmacSecretBytes)
	if err != nil {
		log.Fatal(err)
	}
	services = append(services, *collectionService)

	rorgService, err := rworkspace.CreateService(hmacSecretBytes, db)
	if err != nil {
		log.Fatal(err)
	}
	services = append(services, *rorgService)

	// Start services
	go func() {
		err := api.ListenServices(services, port)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for signal
	<-sc
	cancel()
}

func PrepareTables(db *sql.DB) error {
	err := sresultapi.PrepareTables(db)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func PrepareStatements(db *sql.DB) error {
	// Prepared statements
	err := sresultapi.PrepareStatements(db)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}
