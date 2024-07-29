package main

import (
	"devtools-backend/internal/api"
	"devtools-backend/internal/api/auth"
	"devtools-backend/internal/api/collection"
	"devtools-backend/internal/api/flow"
	"devtools-backend/internal/api/node"
	"devtools-backend/pkg/db/turso"
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

	db, err := turso.NewTurso(dbName, dbUsername, dbToken)
	if err != nil {
		log.Fatal(err)
	}

	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		log.Fatal(errors.New("HMAC_SECRET env var is required"))
	}
	hmacSecretBytes := []byte(hmacSecret)

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

	collectionService, err := collection.CreateService(db, hmacSecretBytes)
	if err != nil {
		log.Fatal(err)
	}
	services = append(services, *collectionService)

	go func() {
		err := api.ListenServices(services, port)
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-sc
}
