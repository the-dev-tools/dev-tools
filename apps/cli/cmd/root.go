package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "devtoolscli",
	Short: "DevTools is a powerful API testing tool",
	Long: `DevTools is a powerful API testing tool that records your browser interactions,
automatically generates requests, and seamlessly chains them for functional testing.
With built-in CI integration, it streamlines API validation from development to deployment.
  `,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
	// Execute prints the error once; cobra would print it a second time.
	SilenceErrors: true,
}

var (
	cfgFilePath string
)

const (
	ConfigFileName      = ".devtools"
	ConfigFileExtension = ".yaml"
)

func init() {
	homePath, err := homedir.Dir()
	if err != nil {
		log.Fatal(err)
	}

	cfgFilePath = fmt.Sprintf("%s/%s%s", homePath, ConfigFileName, ConfigFileExtension)

	viper.SetDefault("data", DefaultConfig{})

	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFilePath, "config", cfgFilePath, "config file (default is $HOME/.devtools.yaml)")
}

// Root returns the CLI's root command, so another program can embed every DevTools command
// (flow run, import, …) in its own binary — Stresseur's `stress` does — instead of running
// this binary as a subprocess. The caller may rename it and add commands before executing it.
func Root() *cobra.Command {
	return rootCmd
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(ExitCode(err))
	}
}

func initConfig() {
	viper.SetConfigType("yaml")
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Error finding home directory: %s", err)
	}

	// Search config in home directory with name ".cobra" (without extension).
	viper.AddConfigPath(home)
	viper.AddConfigPath(cfgFilePath)
	viper.SetConfigName(".devtools")
	// A missing config file is the normal case: nothing requires one. The CLI used to announce
	// "Config file not found" on stdout and write a default ~/.devtools.yaml on every first
	// run, which ended up in the output of programs embedding it (Stresseur's stress).
	err = viper.ReadInConfig()
	if err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			fmt.Fprintf(os.Stderr, "error reading config file: %s\n", err)
		}
	}
}
