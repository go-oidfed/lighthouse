package config

import (
	"reflect"
	"time"

	"github.com/fatih/structs"
	oidfed "github.com/go-oidfed/lib"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/duration"
	"gopkg.in/yaml.v3"
	"tideland.dev/go/slices"

	"github.com/go-oidfed/lighthouse"
	"github.com/go-oidfed/lighthouse/internal/utils"
)

// Endpoints holds configuration for the different possible endpoints.
//
// Environment variables (with prefix LH_ENDPOINTS_):
//   - LH_ENDPOINTS_FETCH_PATH, LH_ENDPOINTS_FETCH_URL, LH_ENDPOINTS_FETCH_STATEMENT_LIFETIME
//   - LH_ENDPOINTS_LIST_PATH, LH_ENDPOINTS_LIST_URL
//   - LH_ENDPOINTS_RESOLVE_PATH, LH_ENDPOINTS_RESOLVE_URL, LH_ENDPOINTS_RESOLVE_*
//   - LH_ENDPOINTS_TRUST_MARK_STATUS_PATH, LH_ENDPOINTS_TRUST_MARK_STATUS_URL
//   - LH_ENDPOINTS_TRUST_MARK_LIST_PATH, LH_ENDPOINTS_TRUST_MARK_LIST_URL
//   - LH_ENDPOINTS_TRUST_MARK_PATH, LH_ENDPOINTS_TRUST_MARK_URL
//   - LH_ENDPOINTS_HISTORICAL_KEYS_PATH, LH_ENDPOINTS_HISTORICAL_KEYS_URL
//   - LH_ENDPOINTS_ENROLL_PATH, LH_ENDPOINTS_ENROLL_URL
//   - LH_ENDPOINTS_ENROLL_REQUEST_PATH, LH_ENDPOINTS_ENROLL_REQUEST_URL
//   - LH_ENDPOINTS_TRUST_MARK_REQUEST_PATH, LH_ENDPOINTS_TRUST_MARK_REQUEST_URL
//   - LH_ENDPOINTS_ENTITY_COLLECTION_PATH, LH_ENDPOINTS_ENTITY_COLLECTION_URL, LH_ENDPOINTS_ENTITY_COLLECTION_*
type Endpoints struct {
	// FetchEndpoint configures the fetch endpoint.
	// Env prefix: LH_ENDPOINTS_FETCH_
	FetchEndpoint fetchEndpointConf `yaml:"fetch" envconfig:"FETCH"`
	// ListEndpoint configures the list endpoint.
	// Env prefix: LH_ENDPOINTS_LIST_
	ListEndpoint lighthouse.EndpointConf `yaml:"list" envconfig:"LIST"`
	// ResolveEndpoint configures the resolve endpoint.
	// Env prefix: LH_ENDPOINTS_RESOLVE_
	ResolveEndpoint resolveEndpointConf `yaml:"resolve" envconfig:"RESOLVE"`
	// TrustMarkStatusEndpoint configures the trust mark status endpoint.
	// Env prefix: LH_ENDPOINTS_TRUST_MARK_STATUS_
	TrustMarkStatusEndpoint lighthouse.EndpointConf `yaml:"trust_mark_status" envconfig:"TRUST_MARK_STATUS"`
	// TrustMarkedEntitiesListingEndpoint configures the trust mark list endpoint.
	// Env prefix: LH_ENDPOINTS_TRUST_MARK_LIST_
	TrustMarkedEntitiesListingEndpoint lighthouse.EndpointConf `yaml:"trust_mark_list" envconfig:"TRUST_MARK_LIST"`
	// TrustMarkEndpoint configures the trust mark endpoint.
	// Env prefix: LH_ENDPOINTS_TRUST_MARK_
	TrustMarkEndpoint trustMarkEndpointConf `yaml:"trust_mark" envconfig:"TRUST_MARK"`
	// HistoricalKeysEndpoint configures the historical keys endpoint.
	// Env prefix: LH_ENDPOINTS_HISTORICAL_KEYS_
	HistoricalKeysEndpoint lighthouse.EndpointConf `yaml:"historical_keys" envconfig:"HISTORICAL_KEYS"`

	// EnrollmentEndpoint configures the enrollment endpoint.
	// Env prefix: LH_ENDPOINTS_ENROLL_
	// Note: checker config is YAML-only
	EnrollmentEndpoint checkedEndpointConf `yaml:"enroll" envconfig:"ENROLL"`
	// EnrollmentRequestEndpoint configures the enrollment request endpoint.
	// Env prefix: LH_ENDPOINTS_ENROLL_REQUEST_
	EnrollmentRequestEndpoint lighthouse.EndpointConf `yaml:"enroll_request" envconfig:"ENROLL_REQUEST"`
	// TrustMarkRequestEndpoint configures the trust mark request endpoint.
	// Env prefix: LH_ENDPOINTS_TRUST_MARK_REQUEST_
	TrustMarkRequestEndpoint lighthouse.EndpointConf `yaml:"trust_mark_request" envconfig:"TRUST_MARK_REQUEST"`
	// EntityCollectionEndpoint configures the entity collection endpoint.
	// Env prefix: LH_ENDPOINTS_ENTITY_COLLECTION_
	EntityCollectionEndpoint collectionEndpointConf `yaml:"entity_collection" envconfig:"ENTITY_COLLECTION"`
}

// checkedEndpointConf holds endpoint configuration with an entity checker.
//
// Environment variables (with prefix from parent, e.g., LH_ENDPOINTS_ENROLL_):
//   - LH_ENDPOINTS_ENROLL_PATH: Endpoint path
//   - LH_ENDPOINTS_ENROLL_URL: Endpoint URL
//
// Note: checker config is YAML-only (too complex for env vars)
type checkedEndpointConf struct {
	lighthouse.EndpointConf `yaml:",inline"`
	// CheckerConfig is the entity checker configuration.
	// YAML only - too complex for env vars
	CheckerConfig lighthouse.EntityCheckerConfig `yaml:"checker" envconfig:"-"`
}

// fetchEndpointConf holds fetch endpoint configuration.
//
// Environment variables (with prefix LH_ENDPOINTS_FETCH_):
//   - LH_ENDPOINTS_FETCH_PATH: Endpoint path
//   - LH_ENDPOINTS_FETCH_URL: Endpoint URL
//   - LH_ENDPOINTS_FETCH_STATEMENT_LIFETIME: Subordinate statement lifetime (e.g., "166h")
type fetchEndpointConf struct {
	lighthouse.EndpointConf `yaml:",inline"`
	// StatementLifetime is the lifetime of subordinate statements.
	// Env: LH_ENDPOINTS_FETCH_STATEMENT_LIFETIME
	StatementLifetime duration.DurationOption `yaml:"statement_lifetime" envconfig:"STATEMENT_LIFETIME"`
}

// resolveEndpointConf holds resolve endpoint configuration.
//
// Environment variables (with prefix LH_ENDPOINTS_RESOLVE_):
//   - LH_ENDPOINTS_RESOLVE_PATH: Endpoint path
//   - LH_ENDPOINTS_RESOLVE_URL: Endpoint URL
//   - LH_ENDPOINTS_RESOLVE_ALLOWED_TRUST_ANCHORS: Allowed trust anchors (comma-separated)
//   - LH_ENDPOINTS_RESOLVE_USE_ENTITY_COLLECTION_ALLOWED_TRUST_ANCHORS: Use collection TAs
//   - LH_ENDPOINTS_RESOLVE_GRACE_PERIOD: Cache grace period (e.g., "1h")
//   - LH_ENDPOINTS_RESOLVE_TIME_ELAPSED_GRACE_FACTOR: Grace factor (0-1)
//   - LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_*: Proactive resolver settings
type resolveEndpointConf struct {
	lighthouse.EndpointConf `yaml:",inline"`
	// AllowedTrustAnchors is the list of allowed trust anchors.
	// Env: LH_ENDPOINTS_RESOLVE_ALLOWED_TRUST_ANCHORS (comma-separated)
	AllowedTrustAnchors []string `yaml:"allowed_trust_anchors" envconfig:"ALLOWED_TRUST_ANCHORS"`
	// UseEntityCollectionAllowedTrustAnchors uses the entity collection's allowed trust anchors.
	// Env: LH_ENDPOINTS_RESOLVE_USE_ENTITY_COLLECTION_ALLOWED_TRUST_ANCHORS
	UseEntityCollectionAllowedTrustAnchors bool `yaml:"use_entity_collection_allowed_trust_anchors" envconfig:"USE_ENTITY_COLLECTION_ALLOWED_TRUST_ANCHORS"`
	// ProactiveResolver configures proactive resolution.
	// Env prefix: LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_
	ProactiveResolver proactiveResolverConf `yaml:"proactive_resolver" envconfig:"PROACTIVE_RESOLVER"`
	// GracePeriod is the cache grace period.
	// Env: LH_ENDPOINTS_RESOLVE_GRACE_PERIOD
	GracePeriod duration.DurationOption `yaml:"grace_period" envconfig:"GRACE_PERIOD"`
	// TimeElapsedGraceFactor is the grace factor for time elapsed.
	// Env: LH_ENDPOINTS_RESOLVE_TIME_ELAPSED_GRACE_FACTOR
	TimeElapsedGraceFactor float64 `yaml:"time_elapsed_grace_factor" envconfig:"TIME_ELAPSED_GRACE_FACTOR"`
}

// proactiveResolverConf holds proactive resolver configuration.
//
// Environment variables (with prefix LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_):
//   - LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_ENABLED: Enable proactive resolver
//   - LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_CONCURRENCY_LIMIT: Max concurrent resolutions
//   - LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_QUEUE_SIZE: Resolution queue size
//   - LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_RESPONSE_STORAGE_DIR: Storage directory
//   - LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_RESPONSE_STORAGE_STORE_JSON: Store JSON
//   - LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_RESPONSE_STORAGE_STORE_JWT: Store JWT
type proactiveResolverConf struct {
	// Enabled enables the proactive resolver.
	// Env: LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_ENABLED
	Enabled bool `yaml:"enabled" envconfig:"ENABLED"`
	// ConcurrencyLimit is the maximum number of concurrent resolutions.
	// Env: LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_CONCURRENCY_LIMIT
	ConcurrencyLimit int `yaml:"concurrency_limit" envconfig:"CONCURRENCY_LIMIT"`
	// QueueSize is the resolution queue size.
	// Env: LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_QUEUE_SIZE
	QueueSize int `yaml:"queue_size" envconfig:"QUEUE_SIZE"`
	// ResponseStorage configures response storage.
	// Env prefix: LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_RESPONSE_STORAGE_
	ResponseStorage struct {
		// Dir is the storage directory.
		// Env: LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_RESPONSE_STORAGE_DIR
		Dir string `yaml:"dir" envconfig:"DIR"`
		// StoreJSON enables storing JSON responses.
		// Env: LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_RESPONSE_STORAGE_STORE_JSON
		StoreJSON bool `yaml:"store_json" envconfig:"STORE_JSON"`
		// StoreJWT enables storing JWT responses.
		// Env: LH_ENDPOINTS_RESOLVE_PROACTIVE_RESOLVER_RESPONSE_STORAGE_STORE_JWT
		StoreJWT bool `yaml:"store_jwt" envconfig:"STORE_JWT"`
	} `yaml:"response_storage" envconfig:"RESPONSE_STORAGE"`
}

func (c *resolveEndpointConf) validate() error {
	if c.ProactiveResolver.Enabled {
		if c.ProactiveResolver.ResponseStorage.Dir == "" {
			return errors.New("response storage directory must be specified if proactive resolver is used")
		}
		if !c.ProactiveResolver.ResponseStorage.StoreJSON && !c.
			ProactiveResolver.ResponseStorage.StoreJWT {
			return errors.New("at least one response storage format must be enabled if proactive resolver is used")
		}
	}
	return nil
}

// collectionEndpointConf holds configuration for the entity collection endpoint.
//
// Environment variables (with prefix LH_ENDPOINTS_ENTITY_COLLECTION_):
//   - LH_ENDPOINTS_ENTITY_COLLECTION_PATH: Endpoint path
//   - LH_ENDPOINTS_ENTITY_COLLECTION_URL: Endpoint URL
//   - LH_ENDPOINTS_ENTITY_COLLECTION_ALLOWED_TRUST_ANCHORS: Allowed trust anchors (comma-separated)
//   - LH_ENDPOINTS_ENTITY_COLLECTION_INTERVAL: Collection interval (e.g., "1h")
//   - LH_ENDPOINTS_ENTITY_COLLECTION_CONCURRENCY_LIMIT: Max concurrent collections
//   - LH_ENDPOINTS_ENTITY_COLLECTION_PAGINATION_LIMIT: Pagination limit
type collectionEndpointConf struct {
	lighthouse.EndpointConf `yaml:",inline"`
	// AllowedTrustAnchors is the list of allowed trust anchors.
	// Env: LH_ENDPOINTS_ENTITY_COLLECTION_ALLOWED_TRUST_ANCHORS (comma-separated)
	AllowedTrustAnchors []string `yaml:"allowed_trust_anchors" envconfig:"ALLOWED_TRUST_ANCHORS"`
	// Interval is the collection interval.
	// Env: LH_ENDPOINTS_ENTITY_COLLECTION_INTERVAL
	Interval duration.DurationOption `yaml:"interval" envconfig:"INTERVAL"`
	// ConcurrencyLimit is the maximum number of concurrent collections.
	// Env: LH_ENDPOINTS_ENTITY_COLLECTION_CONCURRENCY_LIMIT
	ConcurrencyLimit int `yaml:"concurrency_limit" envconfig:"CONCURRENCY_LIMIT"`
	// PaginationLimit is the pagination limit.
	// Env: LH_ENDPOINTS_ENTITY_COLLECTION_PAGINATION_LIMIT
	PaginationLimit int `yaml:"pagination_limit" envconfig:"PAGINATION_LIMIT"`
}

func (c *collectionEndpointConf) validate() error {
	if c.Interval.Duration() == 0 {
		if c.ConcurrencyLimit != 0 {
			log.Warn(
				"entity collection endpoint: concurrency limit is set" +
					" but periodic collection is disabled (no interval set)",
			)
		}
		return nil
	}
	if len(c.AllowedTrustAnchors) == 0 {
		return errors.New("at least one allowed trust anchor must be specified if periodic collection is used")
	}
	return nil
}

// trustMarkEndpointConf holds trust mark endpoint configuration.
//
// Environment variables (with prefix LH_ENDPOINTS_TRUST_MARK_):
//   - LH_ENDPOINTS_TRUST_MARK_PATH: Endpoint path
//   - LH_ENDPOINTS_TRUST_MARK_URL: Endpoint URL
//
// Note: trust_mark_specs is deprecated and YAML-only
type trustMarkEndpointConf struct {
	lighthouse.EndpointConf `yaml:",inline"`
	// TrustMarkSpecs is deprecated. Use the Admin API instead.
	// YAML only - deprecated and complex
	TrustMarkSpecs []extendedTrustMarkSpec `yaml:"trust_mark_specs" envconfig:"-"`
}

type extendedTrustMarkSpec struct {
	CheckerConfig        lighthouse.EntityCheckerConfig `yaml:"checker"`
	oidfed.TrustMarkSpec `yaml:",inline"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface
func (e *extendedTrustMarkSpec) UnmarshalYAML(node *yaml.Node) error {
	type forChecker struct {
		CheckerConfig lighthouse.EntityCheckerConfig `yaml:"checker"`
	}
	mm := e.TrustMarkSpec
	var fc forChecker

	if err := node.Decode(&fc); err != nil {
		return errors.WithStack(err)
	}
	if err := node.Decode(&mm); err != nil {
		return errors.WithStack(err)
	}
	extra := make(map[string]interface{})
	if err := node.Decode(&extra); err != nil {
		return errors.WithStack(err)
	}
	s1 := structs.New(fc)
	s2 := structs.New(mm)
	for _, tag := range utils.FieldTagNames(s1.Fields(), "yaml") {
		delete(extra, tag)
	}
	for _, tag := range utils.FieldTagNames(s2.Fields(), "yaml") {
		delete(extra, tag)
	}
	if len(extra) == 0 {
		extra = nil
	}

	mm.Extra = extra
	e.TrustMarkSpec = mm
	e.CheckerConfig = fc.CheckerConfig
	return nil
}

// validate implements configValidator for trustMarkEndpointConf
func (c *trustMarkEndpointConf) validate() error {
	if len(c.TrustMarkSpecs) > 0 {
		log.Warn("DEPRECATED: 'trust_mark_specs' in config file is deprecated and will be ignored. " +
			"Use the Admin API to manage trust mark specs in the database. " +
			"A migration tool will be provided in a future release.")
	}
	return nil
}

var defaultEndpointConf = Endpoints{
	FetchEndpoint: fetchEndpointConf{
		StatementLifetime: duration.DurationOption(600000 * time.Second),
	},
	ResolveEndpoint: resolveEndpointConf{
		GracePeriod:            duration.DurationOption(time.Hour),
		TimeElapsedGraceFactor: 0.5,
		ProactiveResolver: proactiveResolverConf{
			ConcurrencyLimit: 64,
			QueueSize:        10000,
			ResponseStorage: struct {
				Dir       string `yaml:"dir" envconfig:"DIR"`
				StoreJSON bool   `yaml:"store_json" envconfig:"STORE_JSON"`
				StoreJWT  bool   `yaml:"store_jwt" envconfig:"STORE_JWT"`
			}{
				StoreJWT: true,
			},
		},
	},
}

func (e *Endpoints) validate() error {
	oidfed.ResolverCacheGracePeriod = e.ResolveEndpoint.GracePeriod.Duration()
	oidfed.ResolverCacheLifetimeElapsedGraceFactor = e.ResolveEndpoint.TimeElapsedGraceFactor

	v := reflect.ValueOf(e).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)

		// Get addressable pointer to field if possible
		if fieldVal.CanAddr() {
			ptr := fieldVal.Addr().Interface()

			if validator, ok := ptr.(configValidator); ok {
				if err := validator.validate(); err != nil {
					return errors.Errorf("validation failed for field '%s': %s", t.Field(i).Name, err.Error())
				}
			}
		}
	}

	if e.ResolveEndpoint.ProactiveResolver.Enabled {
		if !e.EntityCollectionEndpoint.IsSet() || e.EntityCollectionEndpoint.Interval.Duration() == 0 {
			return errors.New("entity collection endpoint must be enabled and interval must be set if proactive resolver is enabled")
		}
		if e.ResolveEndpoint.UseEntityCollectionAllowedTrustAnchors {
			e.ResolveEndpoint.AllowedTrustAnchors = e.EntityCollectionEndpoint.AllowedTrustAnchors
		} else {
			if notAllowed := slices.Subtract(
				e.ResolveEndpoint.AllowedTrustAnchors, e.EntityCollectionEndpoint.AllowedTrustAnchors,
			); len(notAllowed) > 0 {
				return errors.Errorf(
					"all the allowed trust anchors for the resolve endpoint"+
						" must also be allowed for the entity collection"+
						" endpoint if proactive resolver is used; the"+
						" following trust anchors are not allowed for the"+
						" entity collection endpoint but on the resolve"+
						" endpoint"+
						": %+q",
					notAllowed,
				)
			}
		}
		if len(e.ResolveEndpoint.AllowedTrustAnchors) == 0 {
			return errors.New(
				"at least one allowed trust anchor must be" +
					" specified for the resolve endpoint if proactive" +
					" resolver is used",
			)
		}
	}
	return nil
}
