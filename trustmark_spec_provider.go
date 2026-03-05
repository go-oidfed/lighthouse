package lighthouse

import (
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/zachmann/go-utils/duration"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// secondsToDurationOption converts seconds (uint) to a duration.DurationOption
func secondsToDurationOption(seconds uint) duration.DurationOption {
	return duration.DurationOption(time.Duration(seconds) * time.Second)
}

// DBTrustMarkSpecProvider implements oidfed.TrustMarkSpecProvider
// by fetching TrustMarkSpecs from the database.
// It is safe for concurrent use as it delegates to the thread-safe storage layer.
type DBTrustMarkSpecProvider struct {
	store model.TrustMarkSpecStore
}

// NewDBTrustMarkSpecProvider creates a new DBTrustMarkSpecProvider.
func NewDBTrustMarkSpecProvider(store model.TrustMarkSpecStore) *DBTrustMarkSpecProvider {
	return &DBTrustMarkSpecProvider{store: store}
}

// GetTrustMarkSpec returns the TrustMarkSpec for the given trust mark type.
// Returns nil if the trust mark type is not found.
func (p *DBTrustMarkSpecProvider) GetTrustMarkSpec(trustMarkType string) *oidfed.TrustMarkSpec {
	if p.store == nil {
		return nil
	}
	spec, err := p.store.GetByType(trustMarkType)
	if err != nil {
		return nil
	}
	return &oidfed.TrustMarkSpec{
		TrustMarkType: spec.TrustMarkType,
		Lifetime:      secondsToDurationOption(spec.Lifetime),
		Ref:           spec.Ref,
		LogoURI:       spec.LogoURI,
		DelegationJWT: spec.DelegationJWT,
		Extra:         spec.AdditionalClaims,
	}
}

// TrustMarkTypes returns all available trust mark types from the database.
func (p *DBTrustMarkSpecProvider) TrustMarkTypes() []string {
	if p.store == nil {
		return nil
	}
	specs, err := p.store.List()
	if err != nil {
		return nil
	}
	types := make([]string, len(specs))
	for i, s := range specs {
		types[i] = s.TrustMarkType
	}
	return types
}
