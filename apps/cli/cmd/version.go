package cmd

import (
	"fmt"
	"os"

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
	Short: "Print the version number of Stresseur CLI (formerly DevTools CLI)",
	Long:  `All software has versions. This is Stresseur CLI's (formerly DevTools CLI)`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(versionLine(os.Args[0], version))
	},
}

// versionLine returns the `version` output. Invoked under a legacy name
// (devtools, devtoolscli, ...) it is byte-identical to the pre-rename output.
func versionLine(arg0, v string) string {
	if invokedAsStresseur(arg0) {
		return fmt.Sprintf("Stresseur CLI %s (formerly DevTools CLI)\n", v)
	}
	return fmt.Sprintf("DevToolsCLI %s\n", v)
}
