package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

func showAll() {
	showLifetimes()
	showSigning()
	showMetadata()
	showConstraints()
	showMetadataPolicies()
	showMetadataPolicyCrit()
	showAuthorityHints()
	showExtraEntityConfig()
	showPublishedTrustMarks()
	showTrustMarkSpecs()
	showTrustMarkIssuers()
	showTrustMarkOwners()
	showTrustAnchors()
	showEndpoints()
}

func showLifetimes() {
	printHeader("Lifetimes")

	ecLifetime, err := storage.GetEntityConfigurationLifetime(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading EC lifetime: %s\n", err)
	} else {
		printValue("Entity configuration lifetime", ecLifetime)
	}

	ssLifetime, err := storage.GetSubordinateStatementLifetime(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading SS lifetime: %s\n", err)
	} else {
		printValue("Subordinate statement lifetime", ssLifetime)
	}
}

func showSigning() {
	printHeader("Signing")

	alg, err := storage.GetSigningAlg(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading signing algorithm: %s\n", err)
	} else {
		printValue("Signing algorithm", alg.String())
	}

	rsaKeyLen, err := storage.GetRSAKeyLen(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading RSA key length: %s\n", err)
	} else {
		printValue("RSA key length", rsaKeyLen)
	}

	rotation, err := storage.GetKeyRotation(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading key rotation: %s\n", err)
	} else {
		printValue("Key rotation enabled", rotation.Enabled)
		printValue("Key rotation interval", time.Duration(rotation.Interval.Duration()))
		printValue("Key rotation overlap", time.Duration(rotation.Overlap.Duration()))
		printValue("Key announcement lead time", time.Duration(rotation.KeyAnnouncementLeadTime.Duration()))
		printValue("Key announcement lead time EC multiplier", rotation.KeyAnnouncementLeadTimeECMultiplier)
	}
}

func showMetadata() {
	printHeader("Entity Metadata")

	metadata, err := storage.GetMetadata(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading metadata: %s\n", err)
		return
	}
	if metadata == nil {
		fmt.Println("  (not set)")
		return
	}
	if metadata.FederationEntity != nil {
		fe := metadata.FederationEntity
		printValue("Display name", fe.DisplayName)
		printValue("Description", fe.Description)
		printValue("Organization name", fe.OrganizationName)
		printValue("Organization URI", fe.OrganizationURI)
		printValue("Logo URI", fe.LogoURI)
		printValue("Policy URI", fe.PolicyURI)
		printValue("Information URI", fe.InformationURI)
		printList("Keywords", fe.Keywords)
		printList("Contacts", fe.Contacts)
		if len(fe.Extra) > 0 {
			fmt.Println("  Extra:")
			for k, v := range fe.Extra {
				fmt.Printf("    %s: %v\n", k, v)
			}
		}
	} else {
		fmt.Println("  (federation_entity metadata not set)")
	}
}

func showConstraints() {
	printHeader("General Constraints")

	cs, err := storage.GetConstraints(backends.KV)
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	if cs == nil {
		fmt.Println("  (not set)")
		return
	}
	if cs.MaxPathLength != nil {
		printValue("Max path length", *cs.MaxPathLength)
	} else {
		printValue("Max path length", "(not set)")
	}
	printList("Allowed entity types", cs.AllowedEntityTypes)
	if cs.NamingConstraints != nil {
		fmt.Println("  Naming constraints:")
		printList("    Permitted", cs.NamingConstraints.Permitted)
		printList("    Excluded", cs.NamingConstraints.Excluded)
	}
}

func showMetadataPolicies() {
	printHeader("General Metadata Policies")

	var policies map[string]any
	found, err := backends.KV.GetAs(
		model.KeyValueScopeSubordinateStatement,
		model.KeyValueKeyMetadataPolicy, &policies,
	)
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	if !found || len(policies) == 0 {
		fmt.Println("  (not set)")
		return
	}
	pretty, _ := json.MarshalIndent(policies, "", "  ")
	fmt.Printf("  %s\n", string(pretty))
}

func showMetadataPolicyCrit() {
	printHeader("Metadata Policy Crit")

	ops, err := storage.GetMetadataPolicyCrit(backends.KV)
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	opStrs := make([]string, len(ops))
	for i, op := range ops {
		opStrs[i] = string(op)
	}
	printList("Crit operators", opStrs)
}

func showAuthorityHints() {
	printHeader("Authority Hints")

	hints, err := storage.GetAuthorityHints(backends.AuthorityHints)
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	printList("Authority hints", hints)
}

func showExtraEntityConfig() {
	printHeader("Extra Entity Configuration Claims")

	claims, err := backends.AdditionalClaims.List()
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	if len(claims) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, c := range claims {
		critStr := ""
		if c.Crit {
			critStr = " [crit]"
		}
		fmt.Printf("  %s%s: %v\n", c.Claim, critStr, c.Value)
	}
}

func showPublishedTrustMarks() {
	printHeader("Published Trust Marks")

	tms, err := backends.PublishedTrustMarks.List()
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	if len(tms) == 0 {
		fmt.Println("  (none)")
		return
	}
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
}

func showTrustMarkSpecs() {
	printHeader("Trust Mark Specs")

	specs, err := backends.TrustMarkSpecs.List()
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	if len(specs) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, s := range specs {
		fmt.Printf("  - Type: %s\n", s.TrustMarkType)
		if s.Lifetime > 0 {
			fmt.Printf("    Lifetime: %ds\n", s.Lifetime)
		}
		if s.Ref != "" {
			fmt.Printf("    Ref: %s\n", s.Ref)
		}
		if s.LogoURI != "" {
			fmt.Printf("    Logo URI: %s\n", s.LogoURI)
		}
		if s.DelegationJWT != "" {
			fmt.Printf("    Delegation JWT: (set, %d chars)\n", len(s.DelegationJWT))
		}
		if s.EligibilityConfig != nil {
			fmt.Printf("    Eligibility mode: %s\n", s.EligibilityConfig.Mode)
			if s.EligibilityConfig.Checker != nil {
				fmt.Printf("    Checker type: %s\n", s.EligibilityConfig.Checker.Type)
			}
		}
	}
}

func showTrustMarkIssuers() {
	printHeader("Trust Mark Issuers")

	types, err := backends.TrustMarkTypes.List()
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	if len(types) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, t := range types {
		if len(t.Issuers) == 0 {
			continue
		}
		fmt.Printf("  %s:\n", t.TrustMarkType)
		for _, iss := range t.Issuers {
			fmt.Printf("    - %s\n", iss.Issuer)
		}
	}
}

func showTrustMarkOwners() {
	printHeader("Trust Mark Owners")

	types, err := backends.TrustMarkTypes.List()
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	if len(types) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, t := range types {
		if t.Owner == nil {
			continue
		}
		fmt.Printf("  %s: %s\n", t.TrustMarkType, t.Owner.EntityID)
	}
}

func showTrustAnchors() {
	printHeader("Trust Anchors")

	tas, err := backends.TrustAnchors.List()
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	if len(tas) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, ta := range tas {
		fmt.Printf("  - Entity ID: %s\n", ta.EntityID)
		fmt.Printf("    JWKS update enabled: %v\n", ta.EnableJWKSUpdate)
		if ta.KeyPollInterval > 0 {
			fmt.Printf("    Key poll interval: %ds\n", ta.KeyPollInterval)
		}
		if ta.JWKSID != nil && ta.JWKS.Keys.Set != nil {
			fmt.Printf("    JWKS: %d key(s)\n", ta.JWKS.Keys.Len())
		} else {
			fmt.Printf("    JWKS: (not set)\n")
		}
	}
}

func showEndpoints() {
	printHeader("Federation Endpoints")

	endpoints, err := backends.FederationEndpoints.List()
	if err != nil {
		fmt.Printf("  Error: %s\n", err)
		return
	}
	if len(endpoints) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, ep := range endpoints {
		path := "(disabled)"
		if ep.Path != nil {
			path = *ep.Path
		}
		url := ""
		if ep.URL != nil {
			url = *ep.URL
		}
		fmt.Printf("  - Type: %s\n", ep.Type)
		fmt.Printf("    Path: %s\n", path)
		if url != "" {
			fmt.Printf("    URL: %s\n", url)
		}
		fmt.Printf("    Auth enabled: %v\n", ep.AuthEnabled)
		if len(ep.AuthTrustAnchors) > 0 {
			fmt.Print("    Auth trust anchors:")
			for _, ta := range ep.AuthTrustAnchors {
				fmt.Printf(" %s", ta.EntityID)
			}
			fmt.Println()
		}
		if ep.Config != "" {
			var raw map[string]any
			if json.Unmarshal([]byte(ep.Config), &raw) == nil {
				pretty, _ := json.MarshalIndent(raw, "    ", "  ")
				fmt.Printf("    Config: %s\n", string(pretty))
			}
		}
	}
}
