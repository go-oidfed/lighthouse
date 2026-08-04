package main

import (
	"encoding/json"
	"fmt"
	"os"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

func sectionConstraints() {
	printHeader("General Subordinate Statement Constraints")

	current, err := storage.GetConstraints(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}
	if current != nil {
		if current.MaxPathLength != nil {
			printValue("Max path length", *current.MaxPathLength)
		} else {
			printValue("Max path length", "(not set)")
		}
		printList("Allowed entity types", current.AllowedEntityTypes)
	} else {
		fmt.Println("  (not set)")
	}

	if migrationCfg != nil && migrationCfg.Federation.Constraints != nil {
		fmt.Println("  Config file hint:")
		if migrationCfg.Federation.Constraints.MaxPathLength != nil {
			printValue("    Max path length", *migrationCfg.Federation.Constraints.MaxPathLength)
		}
		printList("    Allowed entity types", migrationCfg.Federation.Constraints.AllowedEntityTypes)
	}

	if !promptConfirm("Change?") {
		return
	}

	fmt.Println()
	fmt.Println("Provide a JSON file with the constraints, e.g.:")
	fmt.Println(`  {"max_path_length": 2, "allowed_entity_types": ["openid_provider"]}`)
	fmt.Println()

	path := promptFilePath("Constraints JSON file")
	if path == "" {
		fmt.Println("  Skipped.")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("  Error reading file: %s\n", err)
		return
	}
	var cs oidfed.ConstraintSpecification
	if err := json.Unmarshal(data, &cs); err != nil {
		fmt.Printf("  Error parsing JSON: %s\n", err)
		return
	}
	if err := storage.SetConstraints(backends.KV, &cs); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Println("  Constraints saved.")
}

func sectionMetadataPolicies() {
	printHeader("General Metadata Policies")

	var current map[string]any
	found, err := backends.KV.GetAs(
		model.KeyValueScopeSubordinateStatement,
		model.KeyValueKeyMetadataPolicy, &current,
	)
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}
	if found && len(current) > 0 {
		pretty, _ := json.MarshalIndent(current, "  ", "  ")
		fmt.Printf("  %s\n", string(pretty))
	} else {
		fmt.Println("  (not set)")
	}

	if migrationCfg != nil && migrationCfg.Federation.MetadataPolicyFile != "" {
		fmt.Printf("  Config file hint: metadata_policy_file=%s\n", migrationCfg.Federation.MetadataPolicyFile)
	}

	if !promptConfirm("Change?") {
		return
	}

	fmt.Println()
	fmt.Println("Provide a JSON file with the metadata policies, e.g.:")
	fmt.Println(`  {"openid_provider": {"client_id": {"value": "https://example.com"}}}`)
	fmt.Println()

	path := promptFilePath("Metadata policies JSON file")
	if path == "" {
		fmt.Println("  Skipped.")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("  Error reading file: %s\n", err)
		return
	}
	var policies oidfed.MetadataPolicies
	if err := json.Unmarshal(data, &policies); err != nil {
		fmt.Printf("  Error parsing JSON: %s\n", err)
		return
	}
	if err := backends.KV.SetAny(
		model.KeyValueScopeSubordinateStatement,
		model.KeyValueKeyMetadataPolicy, &policies,
	); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Println("  Metadata policies saved.")
}

func sectionMetadataPolicyCrit() {
	printHeader("Metadata Policy Crit Operators")

	current, err := storage.GetMetadataPolicyCrit(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}
	currentStrs := make([]string, len(current))
	for i, op := range current {
		currentStrs[i] = string(op)
	}
	printList("Current crit operators", currentStrs)

	if migrationCfg != nil && len(migrationCfg.Federation.MetadataPolicyCrit) > 0 {
		hint := make([]string, len(migrationCfg.Federation.MetadataPolicyCrit))
		for i, op := range migrationCfg.Federation.MetadataPolicyCrit {
			hint[i] = string(op)
		}
		printList("Config file hint", hint)
	}

	if !promptConfirm("Change?") {
		return
	}

	input := promptList("Crit operators (comma-separated, Enter to keep current, '-' to clear)", currentStrs)

	ops := make([]oidfed.PolicyOperatorName, len(input))
	for i, op := range input {
		ops[i] = oidfed.PolicyOperatorName(op)
	}

	if err := storage.SetMetadataPolicyCrit(backends.KV, ops); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Printf("  Saved %d crit operators.\n", len(ops))
}

// generalSubordinateAdditionalClaim mirrors the KV-stored format used by the
// Admin API for general subordinate additional claims.
type generalSubordinateAdditionalClaim struct {
	ID    int    `json:"id"`
	Claim string `json:"claim"`
	Value any    `json:"value"`
	Crit  bool   `json:"crit"`
}

func sectionSubordinateAdditionalClaims() {
	printHeader("General Subordinate Additional Claims")

	claims, err := loadSubordinateAdditionalClaims()
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

	for {
		fmt.Println()
		fmt.Println("  1. Add/update a claim")
		fmt.Println("  2. Remove a claim")
		fmt.Println("  3. Done")

		choice := promptString("Select", "")
		switch choice {
		case "1":
			addSubordinateAdditionalClaim(claims)
		case "2":
			removeSubordinateAdditionalClaim(claims)
		case "3", "":
			return
		}
	}
}

func loadSubordinateAdditionalClaims() ([]generalSubordinateAdditionalClaim, error) {
	var claims []generalSubordinateAdditionalClaim
	found, err := backends.KV.GetAs(
		model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, &claims,
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return []generalSubordinateAdditionalClaim{}, nil
	}
	return claims, nil
}

func saveSubordinateAdditionalClaims(claims []generalSubordinateAdditionalClaim) error {
	if len(claims) == 0 {
		return backends.KV.Delete(
			model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims,
		)
	}
	return backends.KV.SetAny(
		model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, claims,
	)
}

func nextSubordinateAdditionalClaimID(claims []generalSubordinateAdditionalClaim) int {
	maxID := 0
	for _, c := range claims {
		if c.ID > maxID {
			maxID = c.ID
		}
	}
	return maxID + 1
}

func addSubordinateAdditionalClaim(claims []generalSubordinateAdditionalClaim) {
	name := promptStringRequired("Claim name")

	valueStr := promptStringRequired("Claim value (JSON, e.g. \"string\", 123, true, {\"key\":\"val\"})")
	var value any
	if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
		fmt.Printf("  Error parsing value as JSON: %s\n", err)
		return
	}
	crit := promptBool("Critical?", false)

	for i := range claims {
		if claims[i].Claim == name {
			claims[i].Value = value
			claims[i].Crit = crit
			if err := saveSubordinateAdditionalClaims(claims); err != nil {
				fmt.Printf("  Error writing: %s\n", err)
				return
			}
			fmt.Printf("  Updated claim '%s'.\n", name)
			return
		}
	}

	claims = append(claims, generalSubordinateAdditionalClaim{
		ID:    nextSubordinateAdditionalClaimID(claims),
		Claim: name,
		Value: value,
		Crit:  crit,
	})
	if err := saveSubordinateAdditionalClaims(claims); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Printf("  Claim '%s' saved.\n", name)
}

func removeSubordinateAdditionalClaim(claims []generalSubordinateAdditionalClaim) {
	name := promptStringRequired("Claim name to remove")
	if !promptConfirm(fmt.Sprintf("Remove '%s'?", name)) {
		return
	}
	found := false
	for i, c := range claims {
		if c.Claim == name {
			claims = append(claims[:i], claims[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		fmt.Printf("  Claim '%s' not found.\n", name)
		return
	}
	if err := saveSubordinateAdditionalClaims(claims); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Printf("  Claim '%s' removed.\n", name)
}
