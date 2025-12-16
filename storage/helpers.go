package storage

import (
	"cmp"
	"encoding/json"
	"slices"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/duration"

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

var DefaultSigningAlg = jwa.ES512()

// GetSigningAlg returns the signing algorithm
func GetSigningAlg(kvStorage model.KeyValueStore) (jwa.SignatureAlgorithm, error) {
	if kvStorage == nil {
		return jwa.ES512(), nil
	}
	var algs []SigningAlgWithNbf
	found, err := kvStorage.GetAs(
		model.KeyValueScopeSigning,
		model.KeyValueKeyAlg, &algs,
	)
	if err != nil {
		return jwa.SignatureAlgorithm{}, err
	}
	if !found {
		return DefaultSigningAlg, nil
	}
	slices.SortFunc(
		algs, func(a, b SigningAlgWithNbf) int {
			if a.Nbf == nil && b.Nbf != nil {
				return -1
			}
			if b.Nbf == nil && a.Nbf != nil {
				return 1
			}
			if a.Nbf == nil && b.Nbf == nil {
				return 0
			}
			return cmp.Compare(a.Nbf.UnixNano(), b.Nbf.UnixNano())
		},
	)
	currentIndex := -1
	now := time.Now()
	for i, a := range algs {
		if a.Nbf != nil && a.Nbf.Before(now) {
			currentIndex = i
			break
		}
	}
	if currentIndex == -1 {
		if algs[0].Nbf == nil {
			currentIndex = 0
		} else {
			// Only future algs stored, returning default
			return DefaultSigningAlg, nil
		}
	}
	alg := algs[currentIndex].SigningAlg
	a, ok := jwa.LookupSignatureAlgorithm(alg)
	if !ok {
		return a, errors.Errorf("invalid signing algorithm: %s", alg)
	}
	if err = kvStorage.SetAny(
		model.KeyValueScopeSigning,
		model.KeyValueKeyAlg, algs[currentIndex:],
	); err != nil {
		log.WithError(err).Error("failed to remove expired signing algorithms")
	}
	return a, nil
}

// SigningAlgWithNbf is a signing algorithm with a not-before time used for
// database storage
type SigningAlgWithNbf struct {
	SigningAlg string
	Nbf        *unixtime.Unixtime
}

// SetSigningAlg sets the signing algorithm
func SetSigningAlg(kvStorage model.KeyValueStore, alg SigningAlgWithNbf) error {
	if kvStorage == nil {
		return errors.New("key value store is not set")
	}
	var stored []SigningAlgWithNbf
	_, err := kvStorage.GetAs(model.KeyValueScopeSigning, model.KeyValueKeyAlg, &stored)
	if err != nil {
		return err
	}
	return kvStorage.SetAny(model.KeyValueScopeSigning, model.KeyValueKeyAlg, append(stored, alg))
}

// GetRSAKeyLen returns the RSA key length
func GetRSAKeyLen(kvStorage model.KeyValueStore) (int, error) {
	const d = 2048
	if kvStorage == nil {
		return d, nil
	}
	var l int
	found, err := kvStorage.GetAs(
		model.KeyValueScopeSigning,
		model.KeyValueKeyRSAKeyLen, &l,
	)
	if err != nil {
		return d, err
	}
	if !found {
		l = d
	}
	return l, nil
}

// SetRSAKeyLen sets the RSA key length
func SetRSAKeyLen(kvStorage model.KeyValueStore, rsaKeyLen int) error {
	if kvStorage == nil {
		return errors.New("key value store is not set")
	}
	return kvStorage.SetAny(model.KeyValueScopeSigning, model.KeyValueKeyRSAKeyLen, rsaKeyLen)
}

// GetKeyRotation returns the kms.KeyRotationConfig
func GetKeyRotation(kvStorage model.KeyValueStore) (c kms.KeyRotationConfig, err error) {
	c = kms.KeyRotationConfig{
		Enabled:  false,
		Interval: duration.DurationOption(time.Second * 600000), // a little bit under a week
		Overlap:  duration.DurationOption(time.Hour),
		EntityConfigurationLifetimeFunc: func() (time.Duration, error) {
			return GetEntityConfigurationLifetime(kvStorage)
		},
	}
	if kvStorage == nil {
		return
	}
	_, err = kvStorage.GetAs(
		model.KeyValueScopeSigning,
		model.KeyValueKeyKeyRotation, &c,
	)
	return
}

// SetKeyRotation sets the kms.KeyRotationConfig
func SetKeyRotation(kvStorage model.KeyValueStore, keyRotation kms.KeyRotationConfig) error {
	if kvStorage == nil {
		return errors.New("key value store is not set")
	}
	return kvStorage.SetAny(model.KeyValueScopeSigning, model.KeyValueKeyKeyRotation, keyRotation)
}
