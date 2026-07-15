package main

import (
	"fmt"

	"github.com/go-oidfed/lighthouse/internal/migration"
	"github.com/go-oidfed/lighthouse/storage/model"
)

func sectionPublishedTrustMarks() {
	printHeader("Published Trust Marks")

	tms, err := backends.PublishedTrustMarks.List()
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}
	if len(tms) > 0 {
		for _, tm := range tms {
			fmt.Printf("  - Type: %s\n", tm.TrustMarkType)
			if tm.TrustMarkIssuer != "" {
				fmt.Printf("    Issuer: %s\n", tm.TrustMarkIssuer)
			}
			if tm.TrustMarkJWT != "" {
				fmt.Printf("    JWT: (set, %d chars)\n", len(tm.TrustMarkJWT))
			}
			fmt.Printf("    Refresh: %v\n", tm.Refresh)
			if tm.SelfIssuanceSpec != nil {
				fmt.Printf("    Self-issuance spec:\n")
				fmt.Printf("      Lifetime: %ds\n", tm.SelfIssuanceSpec.Lifetime)
				if tm.SelfIssuanceSpec.Ref != "" {
					fmt.Printf("      Ref: %s\n", tm.SelfIssuanceSpec.Ref)
				}
				if tm.SelfIssuanceSpec.LogoURI != "" {
					fmt.Printf("      Logo URI: %s\n", tm.SelfIssuanceSpec.LogoURI)
				}
				if len(tm.SelfIssuanceSpec.AdditionalClaims) > 0 {
					fmt.Printf("      Additional claims: (%d keys)\n", len(tm.SelfIssuanceSpec.AdditionalClaims))
				}
				fmt.Printf("      Include extra claims in info: %v\n", tm.SelfIssuanceSpec.IncludeExtraClaimsInInfo)
			}
		}
	} else {
		fmt.Println("  (none)")
	}

	if migrationCfg != nil && len(migrationCfg.Federation.TrustMarks) > 0 {
		fmt.Println("  Config file hint:")
		for _, tm := range migrationCfg.Federation.TrustMarks {
			fmt.Printf("    - Type: %s, Issuer: %s\n", tm.TrustMarkType, tm.TrustMarkIssuer)
		}
	}

	for {
		fmt.Println()
		fmt.Println("  1. Add/update a published trust mark")
		fmt.Println("  2. Remove a published trust mark")
		fmt.Println("  3. Done")

		choice := promptString("Select", "")
		switch choice {
		case "1":
			addPublishedTrustMark()
		case "2":
			removePublishedTrustMark()
		case "3", "":
			return
		}
	}
}

func addPublishedTrustMark() {
	tmType := promptStringRequired("Trust mark type")
	issuer := promptString("Trust mark issuer (optional)", "")
	jwt := promptString("Trust mark JWT (optional, paste full JWT)", "")

	refresh := promptBool("Enable refresh?", false)

	var minLifetime, refreshGrace, refreshRate int
	if refresh {
		minLifetime = promptInt("Min lifetime (seconds, 0=default)", 0)
		refreshGrace = int(promptDuration("Refresh grace period", 0).Seconds())
		refreshRate = int(promptDuration("Refresh rate limit", 0).Seconds())
	}

	var selfSpec *model.SelfIssuedTrustMarkSpec
	if promptBool("Configure self-issuance spec?", false) {
		selfSpec = &model.SelfIssuedTrustMarkSpec{
			Lifetime: promptInt("Self-issuance lifetime (seconds)", 0),
			Ref:      promptString("Ref URI (optional)", ""),
			LogoURI:  promptString("Logo URI (optional)", ""),
		}
		if promptBool("Add additional claims?", false) {
			selfSpec.AdditionalClaims = promptAdditionalClaimsMap()
		}
		selfSpec.IncludeExtraClaimsInInfo = promptBool("Include extra claims in info?", false)
	}

	req := model.AddTrustMark{
		TrustMarkType:      tmType,
		TrustMarkIssuer:    issuer,
		TrustMark:          jwt,
		Refresh:            refresh,
		MinLifetime:        minLifetime,
		RefreshGracePeriod: refreshGrace,
		RefreshRateLimit:   refreshRate,
		SelfIssuanceSpec:   selfSpec,
	}

	existing, err := backends.PublishedTrustMarks.Get(tmType)
	if err != nil && existing != nil {
		if _, err := backends.PublishedTrustMarks.Update(tmType, req); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
		fmt.Printf("  Updated '%s'.\n", tmType)
	} else {
		if _, err := backends.PublishedTrustMarks.Create(req); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
		fmt.Printf("  Added '%s'.\n", tmType)
	}
}

func removePublishedTrustMark() {
	tmType := promptStringRequired("Trust mark type to remove")
	if !promptConfirm(fmt.Sprintf("Remove '%s'?", tmType)) {
		return
	}
	if err := backends.PublishedTrustMarks.Delete(tmType); err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	fmt.Printf("  Removed '%s'.\n", tmType)
}

func sectionTrustMarkSpecs() {
	printHeader("Trust Mark Specs")

	specs, err := backends.TrustMarkSpecs.List()
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}
	if len(specs) > 0 {
		for _, s := range specs {
			fmt.Printf("  - Type: %s\n", s.TrustMarkType)
			if s.Lifetime > 0 {
				fmt.Printf("    Lifetime: %ds\n", s.Lifetime)
			}
			if s.Ref != "" {
				fmt.Printf("    Ref: %s\n", s.Ref)
			}
		}
	} else {
		fmt.Println("  (none)")
	}

	if migrationCfg != nil && len(migrationCfg.Endpoints.TrustMark.TrustMarkSpecs) > 0 {
		fmt.Println("  Config file hint:")
		for _, s := range migrationCfg.Endpoints.TrustMark.TrustMarkSpecs {
			fmt.Printf("    - Type: %s\n", s.TrustMarkType)
		}
	}

	for {
		fmt.Println()
		fmt.Println("  1. Add/update a trust mark spec")
		fmt.Println("  2. Remove a trust mark spec")
		fmt.Println("  3. Done")

		choice := promptString("Select", "")
		switch choice {
		case "1":
			addTrustMarkSpec()
		case "2":
			removeTrustMarkSpec()
		case "3", "":
			return
		}
	}
}

func addTrustMarkSpec() {
	tmType := promptTrustMarkType("Trust mark type")
	lifetime := uint(promptInt("Lifetime (seconds, 0=default)", 0))
	ref := promptString("Ref URI (optional)", "")
	logoURI := promptString("Logo URI (optional)", "")
	delegationJWT := promptString("Delegation JWT (optional)", "")

	req := &model.AddTrustMarkSpec{
		TrustMarkType: tmType,
		Lifetime:      lifetime,
		Ref:           ref,
		LogoURI:       logoURI,
		DelegationJWT: delegationJWT,
	}

	if promptBool("Configure eligibility checker?", false) {
		checkerType := promptStringRequired("Checker type")
		checkerConfigPath := promptFilePath("Checker config JSON file (optional)")
		var checkerConfig *model.CheckerConfig
		if checkerConfigPath != "" {
			checkerConfig = &model.CheckerConfig{
				Type:   checkerType,
				Config: map[string]any{},
			}
		} else {
			checkerConfig = &model.CheckerConfig{
				Type: checkerType,
			}
		}
		req.EligibilityConfig = &model.EligibilityConfig{
			Mode:    model.EligibilityModeCustom,
			Checker: checkerConfig,
		}
	}

	existing, err := backends.TrustMarkSpecs.GetByType(tmType)
	if err != nil && existing != nil {
		if _, err := backends.TrustMarkSpecs.Update(tmType, req); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
		fmt.Printf("  Updated '%s'.\n", tmType)
	} else {
		if _, err := backends.TrustMarkSpecs.Create(req); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
		fmt.Printf("  Added '%s'.\n", tmType)
	}
}

func removeTrustMarkSpec() {
	tmType := promptTrustMarkType("Trust mark type to remove")
	if !promptConfirm(fmt.Sprintf("Remove '%s'?", tmType)) {
		return
	}
	if err := backends.TrustMarkSpecs.Delete(tmType); err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	fmt.Printf("  Removed '%s'.\n", tmType)
}

func sectionTrustMarkIssuers() {
	printHeader("Trust Mark Issuers")

	types, err := backends.TrustMarkTypes.List()
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}
	hasIssuers := false
	for _, t := range types {
		if len(t.Issuers) > 0 {
			hasIssuers = true
			fmt.Printf("  %s:\n", t.TrustMarkType)
			for _, iss := range t.Issuers {
				fmt.Printf("    - %s\n", iss.Issuer)
			}
		}
	}
	if !hasIssuers {
		fmt.Println("  (none)")
	}

	if migrationCfg != nil && len(migrationCfg.Federation.TrustMarkIssuers) > 0 {
		fmt.Println("  Config file hint:")
		for tmType, issuers := range migrationCfg.Federation.TrustMarkIssuers {
			fmt.Printf("    %s: %v\n", tmType, issuers)
		}
	}

	for {
		fmt.Println()
		fmt.Println("  1. Add issuer(s) for a trust mark type")
		fmt.Println("  2. Remove an issuer from a trust mark type")
		fmt.Println("  3. Done")

		choice := promptString("Select", "")
		switch choice {
		case "1":
			addTrustMarkIssuer()
		case "2":
			removeTrustMarkIssuer()
		case "3", "":
			return
		}
	}
}

func addTrustMarkIssuer() {
	tmType := promptTrustMarkType("Trust mark type")

	existing, err := backends.TrustMarkTypes.Get(tmType)
	if err != nil && existing == nil {
		desc := promptString("Description (optional)", "")
		if _, err := backends.TrustMarkTypes.Create(model.AddTrustMarkType{
			TrustMarkType: tmType,
			Description:   desc,
		}); err != nil {
			fmt.Printf("  Error creating type: %s\n", err)
			return
		}
	}

	for {
		issuerID := promptStringRequired("Issuer entity ID (Enter empty to finish)")
		if issuerID == "" {
			break
		}
		if _, err := backends.TrustMarkTypes.AddIssuer(tmType, model.AddTrustMarkIssuer{
			Issuer: issuerID,
		}); err != nil {
			fmt.Printf("  Error: %s\n", err)
			continue
		}
		fmt.Printf("  Added issuer '%s' to '%s'.\n", issuerID, tmType)

		if !promptBool("Add another issuer?", true) {
			break
		}
	}
}

func removeTrustMarkIssuer() {
	tmType := promptTrustMarkType("Trust mark type")
	issuerID := promptStringRequired("Issuer entity ID to remove")
	if !promptConfirm(fmt.Sprintf("Remove issuer '%s' from '%s'?", issuerID, tmType)) {
		return
	}
	issuers, err := backends.TrustMarkTypes.ListIssuers(tmType)
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	for _, iss := range issuers {
		if iss.Issuer == issuerID {
			if _, err := backends.TrustMarkTypes.DeleteIssuerByID(tmType, iss.ID); err != nil {
				fmt.Printf("  Error: %s\n", err)
				return
			}
			fmt.Printf("  Removed '%s' from '%s'.\n", issuerID, tmType)
			return
		}
	}
	fmt.Printf("  Issuer '%s' not found for type '%s'.\n", issuerID, tmType)
}

func sectionTrustMarkOwners() {
	printHeader("Trust Mark Owners")

	types, err := backends.TrustMarkTypes.List()
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}
	hasOwners := false
	for _, t := range types {
		if t.Owner != nil {
			hasOwners = true
			fmt.Printf("  %s: %s\n", t.TrustMarkType, t.Owner.EntityID)
		}
	}
	if !hasOwners {
		fmt.Println("  (none)")
	}

	if migrationCfg != nil && len(migrationCfg.Federation.TrustMarkOwners) > 0 {
		fmt.Println("  Config file hint:")
		for tmType, owner := range migrationCfg.Federation.TrustMarkOwners {
			fmt.Printf("    %s: %s\n", tmType, owner.EntityID)
		}
	}

	for {
		fmt.Println()
		fmt.Println("  1. Set owner for a trust mark type")
		fmt.Println("  2. Remove owner from a trust mark type")
		fmt.Println("  3. Done")

		choice := promptString("Select", "")
		switch choice {
		case "1":
			addTrustMarkOwner()
		case "2":
			removeTrustMarkOwner()
		case "3", "":
			return
		}
	}
}

func addTrustMarkOwner() {
	tmType := promptTrustMarkType("Trust mark type")

	existing, err := backends.TrustMarkTypes.Get(tmType)
	if err != nil && existing == nil {
		desc := promptString("Description (optional)", "")
		if _, err := backends.TrustMarkTypes.Create(model.AddTrustMarkType{
			TrustMarkType: tmType,
			Description:   desc,
		}); err != nil {
			fmt.Printf("  Error creating type: %s\n", err)
			return
		}
	}

	ownerEntityID := promptStringRequired("Owner entity ID")

	req := model.AddTrustMarkOwner{
		EntityID: ownerEntityID,
	}

	if promptBool("Provide JWKS file for owner?", false) {
		jwksPath := promptFilePath("JWKS file")
		if jwksPath != "" {
			jwks, err := migration.ParseJWKSFile(jwksPath)
			if err != nil {
				fmt.Printf("  Error parsing JWKS: %s\n", err)
				return
			}
			req.JWKS = *jwks
		}
	}

	existingOwner, err := backends.TrustMarkTypes.GetOwner(tmType)
	if err == nil && existingOwner != nil {
		if _, err := backends.TrustMarkTypes.UpdateOwner(tmType, req); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
		fmt.Printf("  Updated owner for '%s'.\n", tmType)
	} else {
		if _, err := backends.TrustMarkTypes.CreateOwner(tmType, req); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
		fmt.Printf("  Set owner for '%s'.\n", tmType)
	}
}

func removeTrustMarkOwner() {
	tmType := promptTrustMarkType("Trust mark type")
	if !promptConfirm(fmt.Sprintf("Remove owner from '%s'?", tmType)) {
		return
	}
	if err := backends.TrustMarkTypes.DeleteOwner(tmType); err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	fmt.Printf("  Removed owner from '%s'.\n", tmType)
}
