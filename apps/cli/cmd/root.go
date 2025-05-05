package cmd

import (
	"fmt"
	"log"

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
		cmd.Help()
	},
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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("error executing root command: %s", err)
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
	err = viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("Config file not found, creating default config file")

			// Create default config file if it doesn't exist
			home, _ := homedir.Dir()
			defaultConfigFile := home + "/.devtools.yaml"
			err = viper.SafeWriteConfigAs(defaultConfigFile)
			if err != nil {
				fmt.Printf("Error creating default config file: %s\n", err)
			}
		} else {
			fmt.Printf("error reading config file: %s\n", err)
		}
	}
}
