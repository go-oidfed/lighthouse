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
var trustMarkSpecsStorage model.TrustMarkSpecStore

func loadConfig() error {
	if err := config.Load(configFile); err != nil {
		return err
	}
	log.Println("Loaded Config")
	c := config.Get()

	backs, err := config.LoadStorageBackends(c.Storage)
	if err != nil {
		return err
	}
	subordinateStorage = backs.Subordinates
	trustMarkedEntitiesStorage = backs.TrustMarks
	trustMarkSpecsStorage = backs.TrustMarkSpecs
	return nil
}

func rootRunE(_ *cobra.Command, _ []string) error {
	return loadConfig()
}

func main() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "the config file to use")
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
