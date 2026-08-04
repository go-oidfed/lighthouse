package main

import (
	"github.com/go-oidfed/lighthouse/internal/migration"
)

// Re-export shared types and functions for backward compatibility within lhmigrate.
// The actual definitions live in internal/migration.

type migrationConfig = migration.Config

type migrationSigningConf = migration.SigningConf
type migrationFederationConf = migration.FederationConf
type migrationTrustAnchorConf = migration.TrustAnchorConf
type migrationTrustMarkConfig = migration.TrustMarkConfig
type migrationSelfIssuanceSpec = migration.SelfIssuanceSpec
type migrationTrustMarkOwnerConfig = migration.TrustMarkOwnerConfig
type migrationFederationMetadataConf = migration.FederationMetadataConf
type migrationEndpointsConf = migration.EndpointsConf
type migrationSimpleEndpointConf = migration.SimpleEndpointConf
type migrationResolveEndpointConf = migration.ResolveEndpointConf
type migrationProactiveResolverConf = migration.ProactiveResolverConf
type migrationEnrollEndpointConf = migration.EnrollEndpointConf
type migrationCollectionEndpointConf = migration.CollectionEndpointConf
type migrationAuthConf = migration.AuthConf
type migrationFetchEndpointConf = migration.FetchEndpointConf
type migrationTrustMarkEndpointConf = migration.TrustMarkEndpointConf
type migrationTrustMarkSpecConf = migration.TrustMarkSpecConf
type migrationCheckerConfig = migration.CheckerConfig

type migrationSection = migration.Section

const (
	sectionSigning               = migration.SectionSigning
	sectionFederation            = migration.SectionFederation
	sectionTrustMarkSpecs        = migration.SectionTrustMarkSpecs
	sectionTrustMarks            = migration.SectionTrustMarks
	sectionAuthorityHints        = migration.SectionAuthorityHints
	sectionMetadata              = migration.SectionMetadata
	sectionConstraints           = migration.SectionConstraints
	sectionMetadataPolicyCrit    = migration.SectionMetadataPolicyCrit
	sectionMetadataPolicies      = migration.SectionMetadataPolicies
	sectionConfigLifetime        = migration.SectionConfigLifetime
	sectionStatementLifetime     = migration.SectionStatementLifetime
	sectionAlg                   = migration.SectionAlg
	sectionRSAKeyLen             = migration.SectionRSAKeyLen
	sectionKeyRotation           = migration.SectionKeyRotation
	sectionTrustMarkIssuers      = migration.SectionTrustMarkIssuers
	sectionTrustMarkOwners       = migration.SectionTrustMarkOwners
	sectionExtraEntityConfigData = migration.SectionExtraEntityConfigData
	sectionTrustAnchors          = migration.SectionTrustAnchors
	sectionEndpoints             = migration.SectionEndpoints
)

func allSections() []migrationSection {
	return migration.AllSections()
}

func parseSections(s string) ([]migrationSection, error) {
	return migration.ParseSections(s)
}

func parseSkipSections(s string) (map[migrationSection]bool, error) {
	return migration.ParseSkipSections(s)
}

func isValidSection(s migrationSection) bool {
	return migration.IsValidSection(s)
}

func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, sep) {
		p = trimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
