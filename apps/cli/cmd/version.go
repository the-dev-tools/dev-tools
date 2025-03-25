package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

const version = "v0.0.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of DevToolsCLI",
	Long:  `All software has versions. This is DevToolsCLI's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("DevToolsCLI %s\n", version)
	},
}
