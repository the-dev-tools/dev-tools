//go:build cli

package main

import "github.com/the-dev-tools/dev-tools/apps/cli/cmd"

func init() {
	runCLI = cmd.Execute
}
