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
	lighthouse.EndpointConf `yaml:",inline"`
	GracePeriod             duration.DurationOption `yaml:"grace_period"`
	TimeElapsedGraceFactor  float64                 `yaml:"time_elapsed_grace_factor"`
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
	return nil
}
