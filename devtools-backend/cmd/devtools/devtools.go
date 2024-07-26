package main

import (
	"devtools-backend/internal/api"
	"devtools-backend/internal/api/auth"
	"devtools-backend/internal/api/flow"
	"devtools-backend/internal/api/node"
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

	var services []api.Service
	authService, err := auth.CreateService()
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

	flowService, err := flow.CreateService(client)
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
