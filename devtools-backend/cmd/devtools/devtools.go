package main

import (
	"devtools-backend/internal/api"
	"devtools-backend/internal/api/auth"
	"devtools-backend/internal/api/node"
	"log"
	"os"
	"os/signal"
	"syscall"
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

	nodeService, err := node.CreateService()
	if err != nil {
		log.Fatal(err)
	}
	services = append(services, *nodeService)

	go func() {
		err := api.ListenServices(services, port)
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-sc
}
