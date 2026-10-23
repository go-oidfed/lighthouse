package main

import (
	"fmt"
	"time"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

func sectionAuthorityHints() {
	printHeader("Authority Hints")

	current, err := storage.GetAuthorityHints(backends.AuthorityHints)
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}
	printList("Current authority hints", current)

	if migrationCfg != nil && len(migrationCfg.Federation.AuthorityHints) > 0 {
		printList("Config file hint", migrationCfg.Federation.AuthorityHints)
	}

	for {
		fmt.Println()
		fmt.Println("  1. Add an authority hint")
		fmt.Println("  2. Remove an authority hint")
		fmt.Println("  3. Done")

		choice := promptString("Select", "")
		switch choice {
		case "1":
			entityID := promptStringRequired("Entity ID")
			desc := promptString("Description (optional)", "")
			sync := promptJWKSSync()
			if _, err := backends.AuthorityHints.Create(model.AddAuthorityHint{
				EntityID:    entityID,
				Description: desc,
				JWKSSync:    sync,
			}); err != nil {
				fmt.Printf("  Error: %s\n", err)
				continue
			}
			fmt.Printf("  Added '%s'.\n", entityID)

		case "2":
			entityID := promptStringRequired("Entity ID to remove")
			if !promptConfirm(fmt.Sprintf("Remove '%s'?", entityID)) {
				continue
			}
			if err := backends.AuthorityHints.Delete(entityID); err != nil {
				fmt.Printf("  Error: %s\n", err)
				continue
			}
			fmt.Printf("  Removed '%s'.\n", entityID)

		case "3", "":
			return
		}
	}
}

// promptJWKSSync interactively collects an optional outbound JWKS-sync config
// for a superior authority hint. Returns nil when syncing is disabled.
func promptJWKSSync() *model.JWKSSyncConf {
	fmt.Println()
	fmt.Println("  Notify this superior when federation keys rotate (jwks_sync)?")
	mode := model.JWKSSyncMode(promptString("  Mode (none/push/trigger)", string(model.JWKSSyncNone)))
	switch mode {
	case model.JWKSSyncNone, "":
		return nil
	case model.JWKSSyncPush:
		return &model.JWKSSyncConf{
			Mode:        model.JWKSSyncPush,
			JWTLifetime: int64(promptDuration("  Signed-JWKS lifetime", 10*time.Minute).Seconds()),
			Timeout:     int64(promptDuration("  Timeout", 20*time.Second).Seconds()),
		}
	case model.JWKSSyncTrigger:
		return &model.JWKSSyncConf{
			Mode:    model.JWKSSyncTrigger,
			Timeout: int64(promptDuration("  Timeout", 20*time.Second).Seconds()),
		}
	default:
		fmt.Printf("  Invalid mode '%s', not enabling sync.\n", mode)
		return nil
	}
}
