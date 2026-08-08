package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

// version is overwritten at build time via -ldflags -X (see
// apps/cli/taskfile.yaml's build:release task). The literal below is only
// what a plain `go build` without that flag produces (e.g. local dev
// builds), so it intentionally stays a placeholder rather than tracking the
// package version.
var version = "v0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of DevToolsCLI",
	Long:  `All software has versions. This is DevToolsCLI's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("DevToolsCLI %s\n", version)
	},
}
