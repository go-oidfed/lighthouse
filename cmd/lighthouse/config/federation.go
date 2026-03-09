package config

import (
	"encoding/json"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/duration"
	"github.com/zachmann/go-utils/fileutils"
)

// federationConf holds federation configuration.
//
// Environment variables (with prefix LH_FEDERATION_DATA_):
//   - LH_FEDERATION_DATA_AUTHORITY_HINTS: Authority hints (comma-separated)
//   - LH_FEDERATION_DATA_METADATA_POLICY_FILE: Path to metadata policy JSON
//   - LH_FEDERATION_DATA_CRIT: Critical extensions (comma-separated)
//   - LH_FEDERATION_DATA_CONFIGURATION_LIFETIME: Entity config lifetime (e.g., "24h")
//   - LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_*: Metadata fields (see below)
//
// Note: The following fields are YAML-only (too complex for env vars):
//   - trust_anchors, trust_marks, trust_mark_issuers, trust_mark_owners
//   - constraints, metadata_policy_crit, extra_entity_configuration_data
type federationConf struct {
	// TrustAnchors is the list of trust anchors.
	// YAML only - too complex for env vars
	TrustAnchors oidfed.TrustAnchors `yaml:"trust_anchors" envconfig:"-"`
	// AuthorityHints is the list of authority hints.
	// Env: LH_FEDERATION_DATA_AUTHORITY_HINTS (comma-separated)
	AuthorityHints []string `yaml:"authority_hints" envconfig:"AUTHORITY_HINTS"`
	// Metadata holds federation entity metadata.
	// Env prefix: LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_
	Metadata federationMetadataConf `yaml:"federation_entity_metadata" envconfig:"FEDERATION_ENTITY_METADATA"`
	// MetadataPolicyFile is the path to the metadata policy JSON file.
	// Env: LH_FEDERATION_DATA_METADATA_POLICY_FILE
	MetadataPolicyFile string `yaml:"metadata_policy_file" envconfig:"METADATA_POLICY_FILE"`
	// MetadataPolicy is loaded from MetadataPolicyFile at startup.
	// Not configurable via env vars.
	MetadataPolicy *oidfed.MetadataPolicies `yaml:"-" envconfig:"-"`
	// Constraints specifies federation constraints.
	// YAML only - too complex for env vars
	Constraints *oidfed.ConstraintSpecification `json:"constraints,omitempty" envconfig:"-"`
	// CriticalExtensions lists critical JWT extensions.
	// Env: LH_FEDERATION_DATA_CRIT (comma-separated)
	CriticalExtensions []string `json:"crit,omitempty" envconfig:"CRIT"`
	// MetadataPolicyCrit lists critical metadata policy operators.
	// YAML only - too complex for env vars
	MetadataPolicyCrit []oidfed.PolicyOperatorName `json:"metadata_policy_crit,omitempty" envconfig:"-"`
	// TrustMarks configures trust marks for the entity configuration.
	// YAML only - too complex for env vars
	TrustMarks []*oidfed.EntityConfigurationTrustMarkConfig `yaml:"trust_marks" envconfig:"-"`
	// TrustMarkIssuers specifies allowed trust mark issuers.
	// YAML only - too complex for env vars
	TrustMarkIssuers oidfed.AllowedTrustMarkIssuers `yaml:"trust_mark_issuers" envconfig:"-"`
	// TrustMarkOwners specifies trust mark owners.
	// YAML only - too complex for env vars
	TrustMarkOwners oidfed.TrustMarkOwners `yaml:"trust_mark_owners" envconfig:"-"`
	// ExtraEntityConfigurationData holds extra entity configuration data.
	// YAML only - arbitrary map
	ExtraEntityConfigurationData map[string]any `yaml:"extra_entity_configuration_data" envconfig:"-"`

	// ConfigurationLifetime is the lifetime of the entity configuration.
	// Env: LH_FEDERATION_DATA_CONFIGURATION_LIFETIME
	ConfigurationLifetime duration.DurationOption `yaml:"configuration_lifetime" envconfig:"CONFIGURATION_LIFETIME"`
}

// federationMetadataConf holds federation entity metadata.
//
// Environment variables (with prefix LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_):
//   - LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_DISPLAY_NAME: Display name
//   - LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_DESCRIPTION: Description
//   - LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_KEYWORDS: Keywords (comma-separated)
//   - LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_CONTACTS: Contacts (comma-separated)
//   - LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_LOGO_URI: Logo URL
//   - LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_POLICY_URI: Policy URL
//   - LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_INFORMATION_URI: Information URL
//   - LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_ORGANIZATION_NAME: Organization name
//   - LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_ORGANIZATION_URI: Organization URL
//
// Note: extra is YAML-only (arbitrary map)
type federationMetadataConf struct {
	// DisplayName is the display name.
	// Env: LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_DISPLAY_NAME
	DisplayName string `yaml:"display_name" envconfig:"DISPLAY_NAME"`
	// Description is the description.
	// Env: LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_DESCRIPTION
	Description string `yaml:"description" envconfig:"DESCRIPTION"`
	// Keywords is the list of keywords.
	// Env: LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_KEYWORDS (comma-separated)
	Keywords []string `yaml:"keywords" envconfig:"KEYWORDS"`
	// Contacts is the list of contacts.
	// Env: LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_CONTACTS (comma-separated)
	Contacts []string `yaml:"contacts" envconfig:"CONTACTS"`
	// LogoURI is the logo URL.
	// Env: LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_LOGO_URI
	LogoURI string `yaml:"logo_uri" envconfig:"LOGO_URI"`
	// PolicyURI is the policy URL.
	// Env: LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_POLICY_URI
	PolicyURI string `yaml:"policy_uri" envconfig:"POLICY_URI"`
	// InformationURI is the information URL.
	// Env: LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_INFORMATION_URI
	InformationURI string `yaml:"information_uri" envconfig:"INFORMATION_URI"`
	// OrganizationName is the organization name.
	// Env: LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_ORGANIZATION_NAME
	OrganizationName string `yaml:"organization_name" envconfig:"ORGANIZATION_NAME"`
	// OrganizationURI is the organization URL.
	// Env: LH_FEDERATION_DATA_FEDERATION_ENTITY_METADATA_ORGANIZATION_URI
	OrganizationURI string `yaml:"organization_uri" envconfig:"ORGANIZATION_URI"`
	// ExtraFederationEntityMetadata holds extra metadata fields.
	// YAML only - arbitrary map
	ExtraFederationEntityMetadata map[string]any `yaml:"extra" envconfig:"-"`
}

var defaultFederationConf = federationConf{
	ConfigurationLifetime: duration.DurationOption(24 * time.Hour),
}

func (c *federationConf) validate() error {
	if c.MetadataPolicyFile == "" {
		log.Warn("federation conf: metadata_policy_file not set")
	} else {
		policyContent, err := fileutils.ReadFile(c.MetadataPolicyFile)
		if err != nil {
			return errors.Wrap(err, "error reading metadata_policy file")
		}
		if err = json.Unmarshal(policyContent, &c.MetadataPolicy); err != nil {
			return errors.Wrap(err, "error unmarshalling metadata_policy")
		}
	}
	return nil
}
