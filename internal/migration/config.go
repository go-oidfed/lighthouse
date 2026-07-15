package migration

import (
	oidfed "github.com/go-oidfed/lib"
	"github.com/zachmann/go-utils/duration"
	"gopkg.in/yaml.v3"
)

// Config is a config struct that can parse both legacy and current config formats
// for migration purposes. It includes all fields that need to be migrated to the database.
type Config struct {
	Signing    SigningConf    `yaml:"signing"`
	Federation FederationConf `yaml:"federation_data"`
	Endpoints  EndpointsConf  `yaml:"endpoints"`
}

// SigningConf holds signing config values that should be migrated to DB
type SigningConf struct {
	Alg       string `yaml:"alg"`
	RSAKeyLen int    `yaml:"rsa_key_len"`

	KeyRotation struct {
		Enabled                             bool                    `yaml:"enabled"`
		Interval                            duration.DurationOption `yaml:"interval"`
		Overlap                             duration.DurationOption `yaml:"overlap"`
		KeyAnnouncementLeadTime             duration.DurationOption `yaml:"key_announcement_lead_time"`
		KeyAnnouncementLeadTimeECMultiplier float64                 `yaml:"key_announcement_lead_time_ec_multiplier"`
	} `yaml:"key_rotation"`

	AutomaticKeyRollover struct {
		Enabled  bool                    `yaml:"enabled"`
		Interval duration.DurationOption `yaml:"interval"`
	} `yaml:"automatic_key_rollover"`
}

// FederationConf holds federation config values that should be migrated to DB
type FederationConf struct {
	AuthorityHints               []string                        `yaml:"authority_hints"`
	TrustAnchors                 []TrustAnchorConf               `yaml:"trust_anchors"`
	Constraints                  *oidfed.ConstraintSpecification `yaml:"constraints"`
	MetadataPolicyCrit           []oidfed.PolicyOperatorName     `yaml:"metadata_policy_crit"`
	MetadataPolicyFile           string                          `yaml:"metadata_policy_file"`
	ConfigurationLifetime        duration.DurationOption         `yaml:"configuration_lifetime"`
	Metadata                     FederationMetadataConf          `yaml:"federation_entity_metadata"`
	ExtraEntityConfigurationData map[string]any                  `yaml:"extra_entity_configuration_data"`

	TrustMarks       []TrustMarkConfig               `yaml:"trust_marks"`
	TrustMarkIssuers oidfed.AllowedTrustMarkIssuers  `yaml:"trust_mark_issuers"`
	TrustMarkOwners  map[string]TrustMarkOwnerConfig `yaml:"trust_mark_owners"`
}

// TrustAnchorConf holds a trust anchor from the old config file.
type TrustAnchorConf struct {
	EntityID         string                  `yaml:"entity_id"`
	JWKSFile         string                  `yaml:"jwks_file"`
	JWKS             any                     `yaml:"jwks"`
	EnableJWKSUpdate bool                    `yaml:"enable_jwks_update"`
	KeyPollInterval  duration.DurationOption `yaml:"key_poll_interval"`
}

// TrustMarkConfig holds entity configuration trust mark config for migration
type TrustMarkConfig struct {
	TrustMarkType      string                  `yaml:"trust_mark_type"`
	TrustMarkIssuer    string                  `yaml:"trust_mark_issuer"`
	TrustMarkJWT       string                  `yaml:"trust_mark_jwt"`
	Refresh            bool                    `yaml:"refresh"`
	MinLifetime        duration.DurationOption `yaml:"min_lifetime"`
	RefreshGracePeriod duration.DurationOption `yaml:"refresh_grace_period"`
	RefreshRateLimit   duration.DurationOption `yaml:"refresh_rate_limit"`
	SelfIssuanceSpec   *SelfIssuanceSpec       `yaml:"self_issuance_spec"`
}

// SelfIssuanceSpec holds self-issuance specification for trust marks
type SelfIssuanceSpec struct {
	Lifetime                 int            `yaml:"lifetime"`
	Ref                      string         `yaml:"ref"`
	LogoURI                  string         `yaml:"logo_uri"`
	AdditionalClaims         map[string]any `yaml:"additional_claims"`
	IncludeExtraClaimsInInfo bool           `yaml:"include_extra_claims_in_info"`
}

// TrustMarkOwnerConfig holds trust mark owner config for migration
type TrustMarkOwnerConfig struct {
	EntityID string `yaml:"entity_id"`
	JWKS     any    `yaml:"jwks"`
}

// FederationMetadataConf holds federation entity metadata
type FederationMetadataConf struct {
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
func (m *FederationMetadataConf) ToOIDFedMetadata() *oidfed.Metadata {
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

func (m *FederationMetadataConf) isEmpty() bool {
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

// EndpointsConf holds endpoint config values that should be migrated to DB.
type EndpointsConf struct {
	Fetch            FetchEndpointConf      `yaml:"fetch"`
	List             SimpleEndpointConf     `yaml:"list"`
	Resolve          ResolveEndpointConf    `yaml:"resolve"`
	TrustMarkStatus  SimpleEndpointConf     `yaml:"trust_mark_status"`
	TrustMarkList    SimpleEndpointConf     `yaml:"trust_mark_list"`
	TrustMark        TrustMarkEndpointConf  `yaml:"trust_mark"`
	HistoricalKeys   SimpleEndpointConf     `yaml:"historical_keys"`
	Enroll           EnrollEndpointConf     `yaml:"enroll"`
	EnrollRequest    SimpleEndpointConf     `yaml:"enroll_request"`
	TrustMarkRequest SimpleEndpointConf     `yaml:"trust_mark_request"`
	EntityCollection CollectionEndpointConf `yaml:"entity_collection"`
	Auth             AuthConf               `yaml:"auth"`
}

// SimpleEndpointConf holds path/url/auth for a simple endpoint.
type SimpleEndpointConf struct {
	Path             string   `yaml:"path"`
	URL              string   `yaml:"url"`
	AuthEnabled      bool     `yaml:"auth_enabled"`
	AuthTrustAnchors []string `yaml:"auth_trust_anchors"`
}

// ResolveEndpointConf holds resolve endpoint config.
type ResolveEndpointConf struct {
	Path                                   string                  `yaml:"path"`
	URL                                    string                  `yaml:"url"`
	AuthEnabled                            bool                    `yaml:"auth_enabled"`
	AuthTrustAnchors                       []string                `yaml:"auth_trust_anchors"`
	AllowedTrustAnchors                    []string                `yaml:"allowed_trust_anchors"`
	UseEntityCollectionAllowedTrustAnchors bool                    `yaml:"use_entity_collection_allowed_trust_anchors"`
	GracePeriod                            duration.DurationOption `yaml:"grace_period"`
	TimeElapsedGraceFactor                 float64                 `yaml:"time_elapsed_grace_factor"`
	ProactiveResolver                      ProactiveResolverConf   `yaml:"proactive_resolver"`
}

// ProactiveResolverConf holds proactive resolver config.
type ProactiveResolverConf struct {
	Enabled          bool `yaml:"enabled"`
	ConcurrencyLimit int  `yaml:"concurrency_limit"`
	QueueSize        int  `yaml:"queue_size"`
	ResponseStorage  struct {
		Dir       string `yaml:"dir"`
		StoreJSON bool   `yaml:"store_json"`
		StoreJWT  bool   `yaml:"store_jwt"`
	} `yaml:"response_storage"`
}

// EnrollEndpointConf holds enroll endpoint config with entity checker.
type EnrollEndpointConf struct {
	Path             string    `yaml:"path"`
	URL              string    `yaml:"url"`
	AuthEnabled      bool      `yaml:"auth_enabled"`
	AuthTrustAnchors []string  `yaml:"auth_trust_anchors"`
	Checker          yaml.Node `yaml:"checker"`
}

// CollectionEndpointConf holds entity collection endpoint config.
type CollectionEndpointConf struct {
	Path                string                  `yaml:"path"`
	URL                 string                  `yaml:"url"`
	AuthEnabled         bool                    `yaml:"auth_enabled"`
	AuthTrustAnchors    []string                `yaml:"auth_trust_anchors"`
	AllowedTrustAnchors []string                `yaml:"allowed_trust_anchors"`
	Interval            duration.DurationOption `yaml:"interval"`
	ConcurrencyLimit    int                     `yaml:"concurrency_limit"`
	PaginationLimit     int                     `yaml:"pagination_limit"`
}

// AuthConf holds global endpoint auth config.
type AuthConf struct {
	AllRequireAuth bool              `yaml:"all_require_auth"`
	TrustAnchors   []TrustAnchorConf `yaml:"trust_anchors"`
}

// FetchEndpointConf holds fetch endpoint config
type FetchEndpointConf struct {
	Path              string                  `yaml:"path"`
	URL               string                  `yaml:"url"`
	AuthEnabled       bool                    `yaml:"auth_enabled"`
	AuthTrustAnchors  []string                `yaml:"auth_trust_anchors"`
	StatementLifetime duration.DurationOption `yaml:"statement_lifetime"`
}

// TrustMarkEndpointConf holds trust mark endpoint config
type TrustMarkEndpointConf struct {
	Path             string              `yaml:"path"`
	URL              string              `yaml:"url"`
	AuthEnabled      bool                `yaml:"auth_enabled"`
	AuthTrustAnchors []string            `yaml:"auth_trust_anchors"`
	TrustMarkSpecs   []TrustMarkSpecConf `yaml:"trust_mark_specs"`
}

// TrustMarkSpecConf holds trust mark spec config for migration
type TrustMarkSpecConf struct {
	TrustMarkType string         `yaml:"trust_mark_type"`
	Lifetime      uint           `yaml:"lifetime"`
	Ref           string         `yaml:"ref"`
	LogoURI       string         `yaml:"logo_uri"`
	DelegationJWT string         `yaml:"delegation_jwt"`
	Extra         map[string]any `yaml:"-"`

	Checker *CheckerConfig `yaml:"checker"`
}

// CheckerConfig holds entity checker config for migration
type CheckerConfig struct {
	Type   string         `yaml:"type"`
	Config map[string]any `yaml:"config"`
}
