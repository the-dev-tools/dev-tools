package main

import (
	"log"

	"github.com/the-dev-tools/dev-tools/packages/server/cmd/serverrun"
)

func main() {
	if err := serverrun.Run(); err != nil {
		log.Fatal(err)
	}
}
