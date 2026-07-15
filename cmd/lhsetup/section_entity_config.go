package main

import (
	"encoding/json"
	"fmt"
	"os"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

func sectionMetadata() {
	printHeader("Entity Metadata")

	current, err := storage.GetMetadata(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading current value: %s\n", err)
		return
	}
	if current != nil && current.FederationEntity != nil {
		fe := current.FederationEntity
		printValue("Display name", fe.DisplayName)
		printValue("Description", fe.Description)
		printValue("Organization name", fe.OrganizationName)
		printValue("Organization URI", fe.OrganizationURI)
		printValue("Logo URI", fe.LogoURI)
		printValue("Policy URI", fe.PolicyURI)
		printValue("Information URI", fe.InformationURI)
		printList("Keywords", fe.Keywords)
		printList("Contacts", fe.Contacts)
	} else {
		fmt.Println("  (not set)")
	}

	if migrationCfg != nil && migrationCfg.Federation.Metadata.ToOIDFedMetadata() != nil {
		fmt.Println("  Config file hint:")
		m := migrationCfg.Federation.Metadata
		printValue("    Display name", m.DisplayName)
		printValue("    Description", m.Description)
		printValue("    Organization name", m.OrganizationName)
	}

	if !promptConfirm("Change?") {
		return
	}

	fmt.Println()
	fmt.Println("Provide a JSON file with the federation entity metadata.")
	fmt.Println("The file should contain an OIDF federation entity metadata object, e.g.:")
	fmt.Println(`  {"federation_entity": {"display_name": "My Entity", "organization_name": "Org"}}`)
	fmt.Println()

	path := promptFilePath("Metadata JSON file")
	if path == "" {
		fmt.Println("  Skipped.")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("  Error reading file: %s\n", err)
		return
	}
	var metadata oidfed.Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		fmt.Printf("  Error parsing JSON: %s\n", err)
		return
	}
	if err := storage.SetMetadata(backends.KV, &metadata); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Println("  Metadata saved.")
}

func sectionExtraEntityConfig() {
	printHeader("Extra Entity Configuration Claims")

	claims, err := backends.AdditionalClaims.List()
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}
	if len(claims) > 0 {
		for _, c := range claims {
			critStr := ""
			if c.Crit {
				critStr = " [crit]"
			}
			fmt.Printf("  %s%s: %v\n", c.Claim, critStr, c.Value)
		}
	} else {
		fmt.Println("  (none)")
	}

	if migrationCfg != nil && len(migrationCfg.Federation.ExtraEntityConfigurationData) > 0 {
		fmt.Println("  Config file hint:")
		for k, v := range migrationCfg.Federation.ExtraEntityConfigurationData {
			fmt.Printf("    %s: %v\n", k, v)
		}
	}

	for {
		fmt.Println()
		fmt.Println("  1. Add/update a claim")
		fmt.Println("  2. Remove a claim")
		fmt.Println("  3. Done")

		choice := promptString("Select", "")
		switch choice {
		case "1":
			addExtraClaim()
		case "2":
			removeExtraClaim()
		case "3", "":
			return
		}
	}
}

func addExtraClaim() {
	name := promptStringRequired("Claim name")
	valueStr := promptStringRequired("Claim value (JSON, e.g. \"string\", 123, true, {\"key\":\"val\"})")
	var value any
	if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
		fmt.Printf("  Error parsing value as JSON: %s\n", err)
		return
	}
	crit := promptBool("Critical?", false)

	existing, err := backends.AdditionalClaims.Get(name)
	if err != nil && existing != nil {
		fmt.Printf("  Updating existing claim '%s'\n", name)
		if _, err := backends.AdditionalClaims.Update(name, model.AddAdditionalClaim{
			Claim: name, Value: value, Crit: crit,
		}); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
	} else {
		if _, err := backends.AdditionalClaims.Create(model.AddAdditionalClaim{
			Claim: name, Value: value, Crit: crit,
		}); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
	}
	fmt.Printf("  Claim '%s' saved.\n", name)
}

func removeExtraClaim() {
	name := promptStringRequired("Claim name to remove")
	if !promptConfirm(fmt.Sprintf("Remove '%s'?", name)) {
		return
	}
	if err := backends.AdditionalClaims.Delete(name); err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	fmt.Printf("  Claim '%s' removed.\n", name)
}
