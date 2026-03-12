package main

import (
	oidfed "github.com/go-oidfed/lib"
	"github.com/zachmann/go-utils/duration"
)

// migrationConfig is a config struct that can parse both legacy and current config formats
// for migration purposes. It includes all fields that need to be migrated to the database.
type migrationConfig struct {
	Signing    migrationSigningConf    `yaml:"signing"`
	Federation migrationFederationConf `yaml:"federation_data"`
	Endpoints  migrationEndpointsConf  `yaml:"endpoints"`
}

// migrationSigningConf holds signing config values that should be migrated to DB
type migrationSigningConf struct {
	// Current format
	Alg       string `yaml:"alg"`
	RSAKeyLen int    `yaml:"rsa_key_len"`

	KeyRotation struct {
		Enabled  bool                    `yaml:"enabled"`
		Interval duration.DurationOption `yaml:"interval"`
		Overlap  duration.DurationOption `yaml:"overlap"`
	} `yaml:"key_rotation"`

	// Legacy format (automatic_key_rollover)
	AutomaticKeyRollover struct {
		Enabled  bool                    `yaml:"enabled"`
		Interval duration.DurationOption `yaml:"interval"`
	} `yaml:"automatic_key_rollover"`
}

// migrationFederationConf holds federation config values that should be migrated to DB
type migrationFederationConf struct {
	AuthorityHints               []string                        `yaml:"authority_hints"`
	Constraints                  *oidfed.ConstraintSpecification `yaml:"constraints"`
	MetadataPolicyCrit           []oidfed.PolicyOperatorName     `yaml:"metadata_policy_crit"`
	MetadataPolicyFile           string                          `yaml:"metadata_policy_file"`
	ConfigurationLifetime        duration.DurationOption         `yaml:"configuration_lifetime"`
	Metadata                     migrationFederationMetadataConf `yaml:"federation_entity_metadata"`
	ExtraEntityConfigurationData map[string]any                  `yaml:"extra_entity_configuration_data"`

	// Trust mark related configuration
	TrustMarks       []migrationTrustMarkConfig               `yaml:"trust_marks"`
	TrustMarkIssuers oidfed.AllowedTrustMarkIssuers           `yaml:"trust_mark_issuers"`
	TrustMarkOwners  map[string]migrationTrustMarkOwnerConfig `yaml:"trust_mark_owners"`
}

// migrationTrustMarkConfig holds entity configuration trust mark config for migration
// These are trust marks that should be published in the entity's own entity configuration
type migrationTrustMarkConfig struct {
	TrustMarkType      string                     `yaml:"trust_mark_type"`
	TrustMarkIssuer    string                     `yaml:"trust_mark_issuer"`
	TrustMarkJWT       string                     `yaml:"trust_mark_jwt"`
	Refresh            bool                       `yaml:"refresh"`
	MinLifetime        duration.DurationOption    `yaml:"min_lifetime"`
	RefreshGracePeriod duration.DurationOption    `yaml:"refresh_grace_period"`
	RefreshRateLimit   duration.DurationOption    `yaml:"refresh_rate_limit"`
	SelfIssuanceSpec   *migrationSelfIssuanceSpec `yaml:"self_issuance_spec"`
}

// migrationSelfIssuanceSpec holds self-issuance specification for trust marks
type migrationSelfIssuanceSpec struct {
	Lifetime                 int            `yaml:"lifetime"`
	Ref                      string         `yaml:"ref"`
	LogoURI                  string         `yaml:"logo_uri"`
	AdditionalClaims         map[string]any `yaml:"additional_claims"`
	IncludeExtraClaimsInInfo bool           `yaml:"include_extra_claims_in_info"`
}

// migrationTrustMarkOwnerConfig holds trust mark owner config for migration
type migrationTrustMarkOwnerConfig struct {
	EntityID string `yaml:"entity_id"`
	// JWKS is stored as raw interface{} to handle various JWKS formats in config
	JWKS any `yaml:"jwks"`
}

// migrationFederationMetadataConf holds federation entity metadata
type migrationFederationMetadataConf struct {
	DisplayName      string         `yaml:"display_name"`
	Description      string         `yaml:"description"`
	Keywords         []string       `yaml:"keywords"`
	Contacts         []string       `yaml:"contacts"`
	LogoURI          string         `yaml:"logo_uri"`
	PolicyURI        string         `yaml:"policy_uri"`
	InformationURI   string         `yaml:"information_uri"`
	OrganizationName string         `yaml:"organization_name"`
	OrganizationURI  string         `yaml:"organization_uri"`
	Extra            map[string]any `yaml:"extra"`
}

// ToOIDFedMetadata converts the migration metadata to oidfed.Metadata
func (m *migrationFederationMetadataConf) ToOIDFedMetadata() *oidfed.Metadata {
	if m.isEmpty() {
		return nil
	}
	return &oidfed.Metadata{
		FederationEntity: &oidfed.FederationEntityMetadata{
			DisplayName:      m.DisplayName,
			Description:      m.Description,
			Keywords:         m.Keywords,
			Contacts:         m.Contacts,
			LogoURI:          m.LogoURI,
			PolicyURI:        m.PolicyURI,
			InformationURI:   m.InformationURI,
			OrganizationName: m.OrganizationName,
			OrganizationURI:  m.OrganizationURI,
			Extra:            m.Extra,
		},
	}
}

func (m *migrationFederationMetadataConf) isEmpty() bool {
	return m.DisplayName == "" &&
		m.Description == "" &&
		len(m.Keywords) == 0 &&
		len(m.Contacts) == 0 &&
		m.LogoURI == "" &&
		m.PolicyURI == "" &&
		m.InformationURI == "" &&
		m.OrganizationName == "" &&
		m.OrganizationURI == "" &&
		len(m.Extra) == 0
}

// migrationEndpointsConf holds endpoint config values that should be migrated to DB
type migrationEndpointsConf struct {
	Fetch     migrationFetchEndpointConf     `yaml:"fetch"`
	TrustMark migrationTrustMarkEndpointConf `yaml:"trust_mark"`
}

// migrationFetchEndpointConf holds fetch endpoint config
type migrationFetchEndpointConf struct {
	StatementLifetime duration.DurationOption `yaml:"statement_lifetime"`
}

// migrationTrustMarkEndpointConf holds trust mark endpoint config
type migrationTrustMarkEndpointConf struct {
	TrustMarkSpecs []migrationTrustMarkSpecConf `yaml:"trust_mark_specs"`
}

// migrationTrustMarkSpecConf holds trust mark spec config for migration
type migrationTrustMarkSpecConf struct {
	TrustMarkType string         `yaml:"trust_mark_type"`
	Lifetime      uint           `yaml:"lifetime"` // seconds
	Ref           string         `yaml:"ref"`
	LogoURI       string         `yaml:"logo_uri"`
	DelegationJWT string         `yaml:"delegation_jwt"`
	Extra         map[string]any `yaml:"-"` // Will be populated from unknown fields

	// Checker config (legacy - now handled via eligibility_config in DB)
	Checker *migrationCheckerConfig `yaml:"checker"`
}

// migrationCheckerConfig holds entity checker config for migration
type migrationCheckerConfig struct {
	Type   string         `yaml:"type"`
	Config map[string]any `yaml:"config"`
}

// migrationSection represents which sections to migrate
type migrationSection string

const (
	sectionSigning               migrationSection = "signing"
	sectionFederation            migrationSection = "federation"
	sectionTrustMarkSpecs        migrationSection = "trust_mark_specs"
	sectionTrustMarks            migrationSection = "trust_marks"
	sectionAuthorityHints        migrationSection = "authority_hints"
	sectionMetadata              migrationSection = "metadata"
	sectionConstraints           migrationSection = "constraints"
	sectionMetadataPolicyCrit    migrationSection = "metadata_policy_crit"
	sectionMetadataPolicies      migrationSection = "metadata_policies"
	sectionConfigLifetime        migrationSection = "config_lifetime"
	sectionStatementLifetime     migrationSection = "statement_lifetime"
	sectionAlg                   migrationSection = "alg"
	sectionRSAKeyLen             migrationSection = "rsa_key_len"
	sectionKeyRotation           migrationSection = "key_rotation"
	sectionTrustMarkIssuers      migrationSection = "trust_mark_issuers"
	sectionTrustMarkOwners       migrationSection = "trust_mark_owners"
	sectionExtraEntityConfigData migrationSection = "extra_entity_config"
)

// allSections returns all available migration sections
func allSections() []migrationSection {
	return []migrationSection{
		sectionAlg,
		sectionRSAKeyLen,
		sectionKeyRotation,
		sectionConstraints,
		sectionMetadataPolicyCrit,
		sectionMetadataPolicies,
		sectionConfigLifetime,
		sectionStatementLifetime,
		sectionAuthorityHints,
		sectionMetadata,
		sectionExtraEntityConfigData,
		sectionTrustMarkSpecs,
		sectionTrustMarks,
		sectionTrustMarkIssuers,
		sectionTrustMarkOwners,
	}
}

// parseSections parses a comma-separated list of sections
func parseSections(s string) ([]migrationSection, error) {
	if s == "" || s == "all" {
		return allSections(), nil
	}

	parts := splitAndTrim(s, ",")
	sections := make([]migrationSection, 0, len(parts))

	for _, p := range parts {
		sec := migrationSection(p)
		if !isValidSection(sec) {
			return nil, &invalidSectionError{section: p}
		}
		sections = append(sections, sec)
	}
	return sections, nil
}

// parseSkipSections parses sections to skip
func parseSkipSections(s string) (map[migrationSection]bool, error) {
	if s == "" {
		return nil, nil
	}

	parts := splitAndTrim(s, ",")
	skip := make(map[migrationSection]bool, len(parts))

	for _, p := range parts {
		sec := migrationSection(p)
		if !isValidSection(sec) {
			return nil, &invalidSectionError{section: p}
		}
		skip[sec] = true
	}
	return skip, nil
}

func isValidSection(s migrationSection) bool {
	for _, valid := range allSections() {
		if s == valid {
			return true
		}
	}
	return false
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

type invalidSectionError struct {
	section string
}

func (e *invalidSectionError) Error() string {
	return "invalid section: " + e.section
}
