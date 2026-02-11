package main

import (
	"fmt"
	"log"
	"os"

	"github.com/the-dev-tools/dev-tools/packages/server/cmd/serverrun"
)

const (
	EnvDevToolsMode = "DEVTOOLS_MODE"
	ModeServer      = "server"
	ModeCLI         = "cli"
)

// runCLI is set by mode_cli.go when built with the "cli" build tag.
var runCLI func()

func main() {
	switch os.Getenv(EnvDevToolsMode) {
	case ModeCLI:
		if runCLI == nil {
			fmt.Fprintln(os.Stderr, "cli mode is not available in this build; rebuild with: go build -tags cli")
			os.Exit(1)
		}
		runCLI()
	case ModeServer, "":
		if err := serverrun.Run(); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown %s value %q; expected %q or %q\n", EnvDevToolsMode, os.Getenv(EnvDevToolsMode), ModeServer, ModeCLI)
		os.Exit(1)
	}
}
