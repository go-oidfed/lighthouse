package storage

import (
	"sync"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/duration"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TrustMarkConfigProvider manages EntityConfigurationTrustMarkConfig instances
// for the entity configuration. It loads from the PublishedTrustMarksStore,
// converts to library types, and caches the configs for reuse.
// The configs maintain refresh state (backoff, last tried time) so they
// need to persist between entity configuration requests.
type TrustMarkConfigProvider struct {
	store             model.PublishedTrustMarksStore
	entityID          string
	trustMarkEndpoint string
	trustMarkSigner   func() *jwx.TrustMarkSigner

	mu      sync.RWMutex
	configs []*oidfed.EntityConfigurationTrustMarkConfig
	loaded  bool
}

// NewTrustMarkConfigProvider creates a new TrustMarkConfigProvider.
// Parameters:
//   - store: The storage backend for published trust marks
//   - entityID: The entity ID of this lighthouse instance
//   - trustMarkEndpoint: The trust mark endpoint URL (used for self-referential refresh)
//   - trustMarkSigner: A function that returns the current TrustMarkSigner (to support key rotation)
func NewTrustMarkConfigProvider(
	store model.PublishedTrustMarksStore,
	entityID string,
	trustMarkEndpoint string,
	trustMarkSigner func() *jwx.TrustMarkSigner,
) *TrustMarkConfigProvider {
	return &TrustMarkConfigProvider{
		store:             store,
		entityID:          entityID,
		trustMarkEndpoint: trustMarkEndpoint,
		trustMarkSigner:   trustMarkSigner,
	}
}

// GetConfigs returns the trust mark configurations for inclusion in the entity configuration.
// Configs are cached and reused to maintain refresh state.
// Returns nil (not an error) if the store is nil or no trust marks are configured.
func (p *TrustMarkConfigProvider) GetConfigs() ([]*oidfed.EntityConfigurationTrustMarkConfig, error) {
	if p == nil || p.store == nil {
		return nil, nil
	}

	p.mu.RLock()
	if p.loaded {
		configs := p.configs
		p.mu.RUnlock()
		return configs, nil
	}
	p.mu.RUnlock()

	// Need to load configs
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if p.loaded {
		return p.configs, nil
	}

	if err := p.loadConfigs(); err != nil {
		return nil, err
	}

	return p.configs, nil
}

// Invalidate clears the cached configs, forcing a reload on the next GetConfigs call.
// This should be called when trust marks are added, updated, or deleted via the admin API.
func (p *TrustMarkConfigProvider) Invalidate() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.configs = nil
	p.loaded = false
}

// SetTrustMarkEndpoint updates the trust mark endpoint URL.
// This is called when the trust mark endpoint is configured after provider creation.
func (p *TrustMarkConfigProvider) SetTrustMarkEndpoint(endpoint string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.trustMarkEndpoint = endpoint
	// Invalidate cache so configs are reloaded with the new endpoint
	p.configs = nil
	p.loaded = false
}

// loadConfigs loads trust marks from storage and converts them to library types.
// Must be called with write lock held.
func (p *TrustMarkConfigProvider) loadConfigs() error {
	trustMarks, err := p.store.List()
	if err != nil {
		return err
	}

	configs := make([]*oidfed.EntityConfigurationTrustMarkConfig, 0, len(trustMarks))
	for _, tm := range trustMarks {
		config := p.convertToConfig(tm)
		if config == nil {
			continue
		}

		// Get current signer for verification
		var signer *jwx.TrustMarkSigner
		if p.trustMarkSigner != nil {
			signer = p.trustMarkSigner()
		}

		// Verify initializes the config (extracts claims from JWT, sets defaults, etc.)
		if err := config.Verify(p.entityID, p.trustMarkEndpoint, signer); err != nil {
			log.WithError(err).WithField("trust_mark_type", tm.TrustMarkType).
				Warn("Failed to verify trust mark config, skipping")
			continue
		}

		configs = append(configs, config)
	}

	p.configs = configs
	p.loaded = true
	return nil
}

// convertToConfig converts a storage model to a library EntityConfigurationTrustMarkConfig.
func (*TrustMarkConfigProvider) convertToConfig(tm model.PublishedTrustMark) *oidfed.EntityConfigurationTrustMarkConfig {
	config := &oidfed.EntityConfigurationTrustMarkConfig{
		TrustMarkType:   tm.TrustMarkType,
		TrustMarkIssuer: tm.TrustMarkIssuer,
		JWT:             tm.TrustMarkJWT,
		Refresh:         tm.Refresh,
	}

	// Convert int seconds to duration.DurationOption
	if tm.MinLifetime > 0 {
		config.MinLifetime = duration.DurationOption(time.Duration(tm.MinLifetime) * time.Second)
	}
	if tm.RefreshGracePeriod > 0 {
		config.RefreshGracePeriod = duration.DurationOption(time.Duration(tm.RefreshGracePeriod) * time.Second)
	}
	if tm.RefreshRateLimit > 0 {
		config.RefreshRateLimit = duration.DurationOption(time.Duration(tm.RefreshRateLimit) * time.Second)
	}

	// Convert self-issuance spec if present
	if tm.SelfIssuanceSpec != nil {
		config.SelfIssuanceSpec = &oidfed.SelfIssuedTrustMarkSpec{
			TrustMarkSpec: oidfed.TrustMarkSpec{
				TrustMarkType: tm.TrustMarkType,
				Ref:           tm.SelfIssuanceSpec.Ref,
				LogoURI:       tm.SelfIssuanceSpec.LogoURI,
				Extra:         tm.SelfIssuanceSpec.AdditionalClaims,
			},
			IncludeExtraClaimsInInfo: tm.SelfIssuanceSpec.IncludeExtraClaimsInInfo,
		}
		if tm.SelfIssuanceSpec.Lifetime > 0 {
			config.SelfIssuanceSpec.Lifetime = duration.DurationOption(
				time.Duration(tm.SelfIssuanceSpec.Lifetime) * time.Second,
			)
		}
	}

	return config
}
