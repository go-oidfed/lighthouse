package storage

import (
	"encoding/json"
	"time"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// GetMetadata returns the entity configurtion metadata
func GetMetadata(kvStorage model.KeyValueStore) (*oidfed.Metadata, error) {
	if kvStorage == nil {
		return nil, nil
	}
	raw, err := kvStorage.Get(
		model.KeyValueScopeEntityConfiguration,
		model.KeyValueKeyMetadata,
	)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var m oidfed.Metadata
	if err = json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// GetAuthorityHints returns the list of authority hints
func GetAuthorityHints(store model.AuthorityHintsStore) ([]string, error) {
	if store == nil {
		return nil, nil
	}
	rows, err := store.List()
	if err != nil {
		return nil, err
	}
	hints := make([]string, 0, len(rows))
	for _, r := range rows {
		hints = append(hints, r.EntityID)
	}
	return hints, nil
}

// GetEntityConfigurationLifetime returns the entity configuration lifetime
func GetEntityConfigurationLifetime(kvStorage model.KeyValueStore) (time.Duration, error) {
	if kvStorage == nil {
		return 0, nil
	}
	var seconds int
	found, err := kvStorage.GetAs(model.KeyValueScopeEntityConfiguration, model.KeyValueKeyLifetime, &seconds)
	if err != nil {
		return 0, err
	}
	if !found || seconds <= 0 {
		return 24 * time.Hour, nil
	}
	return time.Duration(seconds) * time.Second, nil
}

// GetEntityConfigurationAdditionalClaims returns the entity configuration additional claims
func GetEntityConfigurationAdditionalClaims(store model.AdditionalClaimsStore) (map[string]any, []string, error) {
	extra := make(map[string]any)
	// Load additional claims for entity configuration as Extra
	if store == nil {
		return nil, nil, nil
	}
	rows, err := store.List()
	if err != nil {
		return nil, nil, err
	}
	var crits []string
	for _, row := range rows {
		extra[row.Claim] = row.Value
		if row.Crit {
			crits = append(crits, row.Claim)
		}
	}
	return extra, crits, nil
}
