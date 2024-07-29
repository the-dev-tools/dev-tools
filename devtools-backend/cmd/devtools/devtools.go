package main

import (
	"devtools-backend/internal/api"
	"devtools-backend/internal/api/auth"
	"devtools-backend/internal/api/flow"
	"devtools-backend/internal/api/node"
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

	go func() {
		err := api.ListenServices(services, port)
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-sc
}
