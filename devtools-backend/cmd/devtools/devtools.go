package main

import (
	"devtools-backend/internal/api"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	go api.ListenMasterNodeService()

	<-sc
}
