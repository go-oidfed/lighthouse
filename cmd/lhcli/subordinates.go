package main

import (
	"encoding/json"
	"fmt"

	"github.com/go-oidfed/lib/jwx"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/zachmann/go-utils/fileutils"

	"github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage/model"
)

var subordinatesCmd = &cobra.Command{
	Use:   "subordinates",
	Short: "Manage subordinates",
	Long:  `Manage subordinates`,
}

var subordinatesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a subordinate",
	Long:  `Add a subordinate`,
	Args:  cobra.ExactArgs(1),
	RunE:  addSubordinate,
}
var subordinatesRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a subordinate",
	Long:  `Remove a subordinate`,
	Args:  cobra.ExactArgs(1),
	RunE:  removeSubordinate,
}
var subordinatesBlockCmd = &cobra.Command{
	Use:   "block",
	Short: "Block a subordinate",
	Long:  `Block a subordinate`,
	Args:  cobra.ExactArgs(1),
	RunE:  blockSubordinate,
}
var subordinatesManageRequestsCmd = &cobra.Command{
	Use:   "requests",
	Short: "Manage subordinate requests",
	Long:  "Manage subordinate requests interactively",
	RunE:  manageSubordinateRequests,
}

var entityTypes []string
var onlyIDs bool
var jwksFile string

func init() {
	subordinatesAddCmd.Flags().StringArrayVarP(&entityTypes, "entity_type", "t", []string{}, "entity type")
	subordinatesAddCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "the config file to use")
	subordinatesAddCmd.Flags().StringVarP(
		&jwksFile, "jwks", "k",
		"", "a file containing the entity's public key("+
			"s) in the jwks format; used to verify that the entity's entity"+
			" configuration is signed with a key from this set.",
	)
	subordinatesRemoveCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "the config file to use")
	subordinatesBlockCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "the config file to use")
	subordinatesManageRequestsCmd.Flags().StringVarP(
		&configFile, "config", "c", "config.yaml", "the config file to use",
	)
	subordinatesManageRequestsCmd.Flags().BoolVarP(
		&printFlag, "print", "p", false, "if set only the requests will be printed, no management is triggered",
	)
	subordinatesManageRequestsCmd.Flags().BoolVar(
		&onlyIDs, "only-ids", false, "if set only the entity ids are printed, not all subordinate info",
	)
	subordinatesCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "the config file to use")
	subordinatesCmd.AddCommand(subordinatesAddCmd)
	subordinatesCmd.AddCommand(subordinatesRemoveCmd)
	subordinatesCmd.AddCommand(subordinatesBlockCmd)
	subordinatesCmd.AddCommand(subordinatesManageRequestsCmd)
	rootCmd.AddCommand(subordinatesCmd)
}

func addSubordinate(cmd *cobra.Command, args []string) error {
	if err := loadConfig(); err != nil {
		return err
	}
	if err := subordinateStorage.Load(); err != nil {
		return errors.Wrap(err, "failed to load subordinates from storage")
	}
	var entityJWKS jwx.JWKS
	if jwksFile != "" {
		data, err := fileutils.ReadFile(jwksFile)
		if err != nil {
			return errors.Wrap(err, "failed to read jwks file")
		}
		if err = json.Unmarshal(data, &entityJWKS); err != nil {
			return errors.Wrap(err, "failed to unmarshal jwks file")
		}
	}

	entityID := args[0]

	// We use a TrustResolver to obtain the entity configuration
	// instead of simply fetching the entity configuration,
	// because this will also verify the signature.
	resolver := oidfed.TrustResolver{
		TrustAnchors: oidfed.TrustAnchors{
			{
				EntityID: entityID,
				JWKS:     entityJWKS,
			},
		},
		StartingEntity: entityID,
		Types:          entityTypes,
	}
	chains := resolver.ResolveToValidChainsWithoutVerifyingMetadata()
	if len(chains) == 0 {
		return errors.New("could not obtain or verify entity configuration")
	}
	entityConfig := chains[0][0]
	if len(entityTypes) == 0 {
		entityTypes = entityConfig.Metadata.GuessEntityTypes()
	}
	subEntityTypes := make([]model.SubordinateEntityType, len(entityTypes))
	for i, t := range entityTypes {
		subEntityTypes[i] = model.SubordinateEntityType{EntityType: t}
	}
	info := model.ExtendedSubordinateInfo{
		JWKS: model.NewJWKS(entityConfig.JWKS),
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID:               entityConfig.Subject,
			SubordinateEntityTypes: subEntityTypes,
		},
	}
	if err := subordinateStorage.Add(info); err != nil {
		return errors.Wrap(err, "failed to add subordinate to storage")
	}
	fmt.Println("subordinate added successfully")
	return nil
}

func removeSubordinate(cmd *cobra.Command, args []string) error {
	if err := loadConfig(); err != nil {
		return err
	}
	if err := subordinateStorage.Load(); err != nil {
		return errors.Wrap(err, "failed to load subordinates from storage")
	}

	entityID := args[0]

	if err := subordinateStorage.Delete(entityID); err != nil {
		return errors.Wrap(err, "failed to remove subordinate from storage")
	}
	fmt.Println("subordinate removed successfully")
	return nil
}

func blockSubordinate(cmd *cobra.Command, args []string) error {
	if err := loadConfig(); err != nil {
		return err
	}
	if err := subordinateStorage.Load(); err != nil {
		return errors.Wrap(err, "failed to load subordinates from storage")
	}

	entityID := args[0]

	if err := subordinateStorage.UpdateStatus(entityID, model.StatusBlocked); err != nil {
		return errors.Wrap(err, "failed to block subordinate in storage")
	}
	fmt.Println("subordinate blocked successfully")
	return nil
}

func manageSubordinateRequests(cmd *cobra.Command, _ []string) error {
	if err := loadConfig(); err != nil {
		return err
	}
	if err := subordinateStorage.Load(); err != nil {
		return errors.Wrap(err, "failed to load subordinates from storage")
	}

	pending, err := subordinateStorage.GetByStatus(model.StatusPending)
	if err != nil {
		return errors.Wrap(err, "failed to get pending subordinates")
	}
	if len(pending) == 0 {
		fmt.Println("No pending requests")
		return nil
	}
	if printFlag {
		var str string
		for _, info := range pending {
			printStr := info.EntityID
			if !onlyIDs {
				extended, err := subordinateStorage.Get(info.EntityID)
				if err != nil {
					return err
				}
				printStr, err = stringSubordinateInfo(*extended)
				if err != nil {
					return err
				}
			}
			str += fmt.Sprintf("%s\n", printStr)
		}
		fmt.Printf("The following entities have pending requests:\n%s\n\n", str)
		return nil
	}

	for _, info := range pending {
		printStr := info.EntityID
		if !onlyIDs {
			extended, err := subordinateStorage.Get(info.EntityID)
			if err != nil {
				return err
			}
			printStr, err = stringSubordinateInfo(*extended)
			if err != nil {
				return err
			}
			if err = promptInSubordinateRequest(info.EntityID, printStr); err != nil {
				return err
			}
		}
	}
	return nil
}

func promptInSubordinateRequest(entityID, str string) error {
	approved := promptApproval("Do you approve entity '%s'", str)
	if approved {
		return subordinateStorage.UpdateStatus(entityID, model.StatusActive)
	}
	return subordinateStorage.UpdateStatus(entityID, model.StatusBlocked)
}

func stringSubordinateInfo(info model.ExtendedSubordinateInfo) (string, error) {
	data, err := json.Marshal(info)
	if err != nil {
		return "", err
	}
	var generic map[string]interface{}
	if err = json.Unmarshal(data, &generic); err != nil {
		return "", err
	}
	delete(generic, "status")
	data, err = json.MarshalIndent(generic, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}
