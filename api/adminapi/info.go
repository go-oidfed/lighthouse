package adminapi

import (
	"reflect"

	"github.com/gofiber/fiber/v3"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/internal/version"
)

// infoFeatures are the feature names GET /info can list, in the order it
// lists them. The enum of the "features" items in openapi.yaml is this list.
var infoFeatures = []string{
	"conditional_status",
	"events",
	"trust_mark_issuance",
	"trust_mark_instances",
	"enroll_check",
	"service_tokens",
	"actor_delegation",
}

// features lists the optional parts of the admin API that are usable now.
// instances reports whether the issued trust mark instance store exists.
func (o *Options) features(instances bool) []string {
	features := []string{"conditional_status", "events"}
	// Eager issuance answers 503 issuance_unavailable without a signer or
	// without the instance store, which records what it issues.
	if o.trustMarkSigner() != nil && instances {
		features = append(features, "trust_mark_issuance")
	}
	if instances {
		features = append(features, "trust_mark_instances")
	}
	// The checker is asked on each request, as endpoints can be reloaded.
	if checker := o.enrollChecker(); checker != nil && checker.Configured() {
		features = append(features, "enroll_check")
	}
	if tokens := o.serviceTokens(); len(tokens) > 0 {
		features = append(features, "service_tokens")
		if tokens.ActForUsers() {
			features = append(features, "actor_delegation")
		}
	}
	return features
}

// infoResponse is the answer of GET /info.
type infoResponse struct {
	Version  string   `json:"version"`
	EntityID string   `json:"entity_id,omitempty"`
	Features []string `json:"features"`
}

// isNil reports whether e is nil or an interface holding a nil pointer.
func isNil(e oidfed.FederationEntity) bool {
	if e == nil {
		return true
	}
	v := reflect.ValueOf(e)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// registerInfo registers GET /info, which tells a client the version and
// which optional parts of the admin API this instance offers. It reports the
// entity ID when the admin API has the running entity.
func registerInfo(r fiber.Router, opts *Options, fedEntity oidfed.FederationEntity, instances bool) {
	r.Get(
		"/info", func(c fiber.Ctx) error {
			info := infoResponse{Version: version.VERSION, Features: opts.features(instances)}
			if !isNil(fedEntity) {
				info.EntityID = fedEntity.EntityID()
			}
			return c.JSON(info)
		},
	)
}
