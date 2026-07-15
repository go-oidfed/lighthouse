package main

import (
	"fmt"

	"github.com/go-oidfed/lighthouse/internal/migration"
	"github.com/go-oidfed/lighthouse/storage/model"
)

func sectionTrustAnchors() {
	printHeader("Trust Anchors")

	tas, err := backends.TrustAnchors.List()
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}
	if len(tas) > 0 {
		for _, ta := range tas {
			fmt.Printf("  - %s\n", ta.EntityID)
			fmt.Printf("    JWKS update: %v\n", ta.EnableJWKSUpdate)
			if ta.KeyPollInterval > 0 {
				fmt.Printf("    Key poll interval: %ds\n", ta.KeyPollInterval)
			}
		}
	} else {
		fmt.Println("  (none)")
	}

	if migrationCfg != nil && len(migrationCfg.Federation.TrustAnchors) > 0 {
		fmt.Println("  Config file hint:")
		for _, ta := range migrationCfg.Federation.TrustAnchors {
			fmt.Printf("    - %s\n", ta.EntityID)
		}
	}

	for {
		fmt.Println()
		fmt.Println("  1. Add/update a trust anchor")
		fmt.Println("  2. Remove a trust anchor")
		fmt.Println("  3. Done")

		choice := promptString("Select", "")
		switch choice {
		case "1":
			addTrustAnchor()
		case "2":
			removeTrustAnchor()
		case "3", "":
			return
		}
	}
}

func addTrustAnchor() {
	entityID := promptStringRequired("Entity ID")

	jwksPath := promptFilePath("JWKS file (optional, Enter to skip)")

	enableUpdate := promptBool("Enable JWKS auto-update?", false)

	var keyPollInterval int64
	if enableUpdate {
		keyPollInterval = int64(promptDuration("Key poll interval (0 = derive from EC expiration)", 0).Seconds())
	}

	var jwks *model.JWKS
	if jwksPath != "" {
		j, err := migration.ParseJWKSFile(jwksPath)
		if err != nil {
			fmt.Printf("  Error parsing JWKS: %s\n", err)
			return
		}
		jwks = j
	}

	req := model.AddTrustAnchor{
		EntityID:         entityID,
		JWKS:             jwks,
		EnableJWKSUpdate: enableUpdate,
		KeyPollInterval:  keyPollInterval,
	}

	existing, err := backends.TrustAnchors.Get(entityID)
	if err != nil && existing != nil {
		if _, err := backends.TrustAnchors.Update(entityID, req); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
		fmt.Printf("  Updated '%s'.\n", entityID)
	} else {
		if _, err := backends.TrustAnchors.Create(req); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
		fmt.Printf("  Added '%s'.\n", entityID)
	}
}

func removeTrustAnchor() {
	entityID := promptStringRequired("Entity ID to remove")
	if !promptConfirm(fmt.Sprintf("Remove '%s'?", entityID)) {
		return
	}
	if err := backends.TrustAnchors.Delete(entityID); err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	fmt.Printf("  Removed '%s'.\n", entityID)
}
