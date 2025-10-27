package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
	"github.com/go-oidfed/lighthouse/storage/model"
)

var rootCmd = &cobra.Command{
	Use:   "lhcli",
	Short: "lhcli can help you manage your LightHouse",
	Long:  "lhcli can help you manage your LightHouse",
	RunE:  rootRunE,
}

var configFile string
var subordinateStorage model.SubordinateStorageBackend
var trustMarkedEntitiesStorage model.TrustMarkedEntitiesStorageBackend

func loadConfig() error {
	config.Load(configFile)
	log.Println("Loaded Config")
	c := config.Get()

	var err error
	backs, err := config.LoadStorageBackends(c.Storage)
	if err != nil {
		log.Fatal(err)
	}
	subordinateStorage = backs.Subordinates
	trustMarkedEntitiesStorage = backs.TrustMarks
	return nil
}

func rootRunE(cmd *cobra.Command, args []string) error {
	return loadConfig()
}

func main() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "the config file to use")
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
