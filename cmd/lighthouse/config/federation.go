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

type federationConf struct {
	EntityID                     string                                       `yaml:"entity_id"`
	TrustAnchors                 oidfed.TrustAnchors                          `yaml:"trust_anchors"`
	AuthorityHints               []string                                     `yaml:"authority_hints"`
	Metadata                     federationMetadataConf                       `yaml:"federation_entity_metadata"`
	MetadataPolicyFile           string                                       `yaml:"metadata_policy_file"`
	MetadataPolicy               *oidfed.MetadataPolicies                     `yaml:"-"`
	Constraints                  *oidfed.ConstraintSpecification              `json:"constraints,omitempty"`
	CriticalExtensions           []string                                     `json:"crit,omitempty"`
	MetadataPolicyCrit           []oidfed.PolicyOperatorName                  `json:"metadata_policy_crit,omitempty"`
	TrustMarks                   []*oidfed.EntityConfigurationTrustMarkConfig `yaml:"trust_marks"`
	TrustMarkIssuers             oidfed.AllowedTrustMarkIssuers               `yaml:"trust_mark_issuers"`
	TrustMarkOwners              oidfed.TrustMarkOwners                       `yaml:"trust_mark_owners"`
	ExtraEntityConfigurationData map[string]any                               `yaml:"extra_entity_configuration_data"`

	ConfigurationLifetime duration.DurationOption `yaml:"configuration_lifetime"`
}

type federationMetadataConf struct {
	DisplayName                   string         `yaml:"display_name"`
	Description                   string         `yaml:"description"`
	Keywords                      []string       `yaml:"keywords"`
	Contacts                      []string       `yaml:"contacts"`
	LogoURI                       string         `yaml:"logo_uri"`
	PolicyURI                     string         `yaml:"policy_uri"`
	InformationURI                string         `yaml:"information_uri"`
	OrganizationName              string         `yaml:"organization_name"`
	OrganizationURI               string         `yaml:"organization_uri"`
	ExtraFederationEntityMetadata map[string]any `yaml:"extra"`
}

var defaultFederationConf = federationConf{
	ConfigurationLifetime: duration.DurationOption(24 * time.Hour),
}

func (c *federationConf) validate() error {
	if c.EntityID == "" {
		return errors.New("error in federation conf: entity_id must be specified")
	}
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
