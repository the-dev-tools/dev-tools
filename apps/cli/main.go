package main

import (
	"fmt"
	"log"
	"os"

	"github.com/the-dev-tools/dev-tools/packages/server/cmd/serverrun"
)

const (
	EnvDevToolsMode = "DEVTOOLS_MODE"
	// EnvStresseurMode is the new name for EnvDevToolsMode. Both are accepted;
	// when both are set (non-empty), STRESSEUR_MODE wins.
	EnvStresseurMode = "STRESSEUR_MODE"
	ModeServer       = "server"
	ModeCLI          = "cli"
)

// runCLI is set by mode_cli.go when built with the "cli" build tag.
var runCLI func()

// resolveMode returns the requested run mode and the environment variable that
// supplied it. STRESSEUR_MODE takes precedence over DEVTOOLS_MODE; an empty
// value counts as unset (as DEVTOOLS_MODE always has). With neither set it
// returns ("", EnvDevToolsMode).
func resolveMode(lookup func(string) (string, bool)) (value, varName string) {
	if v, ok := lookup(EnvStresseurMode); ok && v != "" {
		return v, EnvStresseurMode
	}
	v, _ := lookup(EnvDevToolsMode)
	return v, EnvDevToolsMode
}

func main() {
	mode, modeVar := resolveMode(os.LookupEnv)
	switch mode {
	case ModeCLI:
		if runCLI == nil {
			fmt.Fprintln(os.Stderr, "cli mode is not available in this build; rebuild with: go build -tags cli")
			os.Exit(1)
		}
		runCLI()
	case ModeServer:
		if err := serverrun.Run(); err != nil {
			log.Fatal(err)
		}
	case "":
		// No explicit mode — prefer CLI when the binary was built with the `cli` tag
		// (runCLI is wired by mode_cli.go). Server-only builds (no tag) fall back to server
		// mode so Electron's bundled server binary keeps working without DEVTOOLS_MODE set.
		if runCLI != nil {
			runCLI()
			return
		}
		if err := serverrun.Run(); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown %s value %q; expected %q or %q\n", modeVar, mode, ModeServer, ModeCLI)
		os.Exit(1)
	}
}
