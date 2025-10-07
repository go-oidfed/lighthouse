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

// Endpoints holds configuration for the different possible endpoints
type Endpoints struct {
	FetchEndpoint                      fetchEndpointConf       `yaml:"fetch"`
	ListEndpoint                       lighthouse.EndpointConf `yaml:"list"`
	ResolveEndpoint                    resolveEndpointConf     `yaml:"resolve"`
	TrustMarkStatusEndpoint            lighthouse.EndpointConf `yaml:"trust_mark_status"`
	TrustMarkedEntitiesListingEndpoint lighthouse.EndpointConf `yaml:"trust_mark_list"`
	TrustMarkEndpoint                  trustMarkEndpointConf   `yaml:"trust_mark"`
	HistoricalKeysEndpoint             lighthouse.EndpointConf `yaml:"historical_keys"`

	EnrollmentEndpoint        checkedEndpointConf     `yaml:"enroll"`
	EnrollmentRequestEndpoint lighthouse.EndpointConf `yaml:"enroll_request"`
	TrustMarkRequestEndpoint  lighthouse.EndpointConf `yaml:"trust_mark_request"`
	EntityCollectionEndpoint  collectionEndpointConf  `yaml:"entity_collection"`
}

type checkedEndpointConf struct {
	lighthouse.EndpointConf `yaml:",inline"`
	CheckerConfig           lighthouse.EntityCheckerConfig `yaml:"checker"`
}

type fetchEndpointConf struct {
	lighthouse.EndpointConf `yaml:",inline"`
	StatementLifetime       duration.DurationOption `yaml:"statement_lifetime"`
}

type resolveEndpointConf struct {
	lighthouse.EndpointConf                `yaml:",inline"`
	AllowedTrustAnchors                    []string                `yaml:"allowed_trust_anchors"`
	UseEntityCollectionAllowedTrustAnchors bool                    `yaml:"use_entity_collection_allowed_trust_anchors"`
	ProactiveResolver                      proactiveResolverConf   `yaml:"proactive_resolver"`
	GracePeriod                            duration.DurationOption `yaml:"grace_period"`
	TimeElapsedGraceFactor                 float64                 `yaml:"time_elapsed_grace_factor"`
}

type proactiveResolverConf struct {
	Enabled          bool `yaml:"enabled"`
	ConcurrencyLimit int  `yaml:"concurrency_limit"`
	QueueSize        int  `yaml:"queue_size"`
	ResponseStorage  struct {
		Dir       string `yaml:"dir"`
		StoreJSON bool   `yaml:"store_json"`
		StoreJWT  bool   `yaml:"store_jwt"`
	} `yaml:"response_storage"`
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

// collectionEndpointConf holds configuration for the entity collection endpoint
type collectionEndpointConf struct {
	lighthouse.EndpointConf `yaml:",inline"`
	AllowedTrustAnchors     []string                `yaml:"allowed_trust_anchors"`
	Interval                duration.DurationOption `yaml:"interval"`
	ConcurrencyLimit        int                     `yaml:"concurrency_limit"`
	PaginationLimit         int                     `yaml:"pagination_limit"`
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

type trustMarkEndpointConf struct {
	lighthouse.EndpointConf `yaml:",inline"`
	TrustMarkSpecs          []extendedTrustMarkSpec `yaml:"trust_mark_specs"`
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
	e.IncludeExtraClaimsInInfo = true
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
				Dir       string `yaml:"dir"`
				StoreJSON bool   `yaml:"store_json"`
				StoreJWT  bool   `yaml:"store_jwt"`
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
