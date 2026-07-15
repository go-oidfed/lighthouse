package main

import (
	"fmt"

	"github.com/go-oidfed/lighthouse/internal/migration"
)

func runWizard(sections []migration.Section) {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  Lighthouse Interactive Setup")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("This utility will prompt you for each DB-managed")
	fmt.Println("configuration option. Press Enter to keep the current")
	fmt.Println("value. Values from a config file (if provided) are shown")
	fmt.Println("as hints where they differ from the DB.")

	for _, section := range sections {
		switch section {
		case migration.SectionConfigLifetime:
			sectionConfigLifetime()
		case migration.SectionStatementLifetime:
			sectionStatementLifetime()
		case migration.SectionAlg:
			sectionSigningAlg()
		case migration.SectionRSAKeyLen:
			sectionRSAKeyLen()
		case migration.SectionKeyRotation:
			sectionKeyRotation()
		case migration.SectionMetadata:
			sectionMetadata()
		case migration.SectionConstraints:
			sectionConstraints()
		case migration.SectionMetadataPolicies:
			sectionMetadataPolicies()
		case migration.SectionMetadataPolicyCrit:
			sectionMetadataPolicyCrit()
		case migration.SectionAuthorityHints:
			sectionAuthorityHints()
		case migration.SectionExtraEntityConfigData:
			sectionExtraEntityConfig()
		case migration.SectionTrustMarks:
			sectionPublishedTrustMarks()
		case migration.SectionTrustMarkSpecs:
			sectionTrustMarkSpecs()
		case migration.SectionTrustMarkIssuers:
			sectionTrustMarkIssuers()
		case migration.SectionTrustMarkOwners:
			sectionTrustMarkOwners()
		case migration.SectionTrustAnchors:
			sectionTrustAnchors()
		case migration.SectionEndpoints:
			sectionEndpoints()
		}
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  Setup complete!")
	fmt.Println("========================================")
}
