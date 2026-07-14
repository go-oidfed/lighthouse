package lighthouse

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/duration"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TrustAnchorRepo is the in-memory, single-source-of-truth repository for
// trust anchors. It holds canonical *oidfed.TrustAnchor instances keyed by
// entity_id, loaded from the database at startup.
//
// All consumers across LightHouse (client auth middleware, entity checkers,
// allowed-trust-anchor lists, ...) resolve TAs by entity_id through this repo
// so that JWKS refreshed by the TAJWKSRefresher propagate live to every usage
// without restart.
type TrustAnchorRepo struct {
	mu      sync.RWMutex
	anchors map[string]*oidfed.TrustAnchor
	store   model.TrustAnchorStore
}

// NewTrustAnchorRepo creates a new TrustAnchorRepo backed by the given store.
// Call Load() to populate it from the database.
func NewTrustAnchorRepo(store model.TrustAnchorStore) *TrustAnchorRepo {
	return &TrustAnchorRepo{
		anchors: make(map[string]*oidfed.TrustAnchor),
		store:   store,
	}
}

// Load (re)loads all trust anchors from the database into the in-memory map.
// Existing *oidfed.TrustAnchor pointers are preserved when the entity still
// exists (so concurrent readers holding a pointer see the JWKS update in
// place); entries for removed TAs are dropped.
func (r *TrustAnchorRepo) Load() error {
	items, err := r.store.List()
	if err != nil {
		return errors.Wrap(err, "trust_anchor_repo: load failed")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Keep existing pointers where possible; add new; remove gone.
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		ta := r.taFromModel(&item)
		if existing, ok := r.anchors[item.EntityID]; ok {
			// Preserve the pointer, update its JWKS in place (atomic).
			existing.SetJWKS(ta.JWKS())
			existing.EnableJWKSUpdate = ta.EnableJWKSUpdate
			existing.KeyPollInterval = ta.KeyPollInterval
		} else {
			r.anchors[item.EntityID] = ta
		}
		seen[item.EntityID] = true
	}
	for id := range r.anchors {
		if !seen[id] {
			delete(r.anchors, id)
		}
	}
	log.WithField("count", len(r.anchors)).Debug("TrustAnchorRepo loaded")
	return nil
}

// taFromModel converts a model.TrustAnchor to an *oidfed.TrustAnchor.
func (r *TrustAnchorRepo) taFromModel(m *model.TrustAnchor) *oidfed.TrustAnchor {
	ta := &oidfed.TrustAnchor{
		EntityID:         m.EntityID,
		EnableJWKSUpdate: m.EnableJWKSUpdate,
		KeyPollInterval:  duration.DurationOption(time.Duration(m.KeyPollInterval) * time.Second),
	}
	if m.JWKSID != nil && m.JWKS.Keys.Set != nil {
		ta.SetJWKS(m.JWKS.Keys)
	}
	return ta
}

// Get returns the *oidfed.TrustAnchor for the given entity_id, or nil if not found.
func (r *TrustAnchorRepo) Get(entityID string) *oidfed.TrustAnchor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.anchors[entityID]
}

// Resolve returns oidfed.TrustAnchors for the given entity ids, skipping any
// that are not in the repository.
func (r *TrustAnchorRepo) Resolve(entityIDs ...string) oidfed.TrustAnchors {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out oidfed.TrustAnchors
	for _, id := range entityIDs {
		if ta, ok := r.anchors[id]; ok {
			out = append(out, ta)
		}
	}
	return out
}

// All returns all trust anchors currently in the repository.
func (r *TrustAnchorRepo) All() oidfed.TrustAnchors {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(oidfed.TrustAnchors, 0, len(r.anchors))
	for _, ta := range r.anchors {
		out = append(out, ta)
	}
	return out
}

// AllWithJWKSUpdate returns all trust anchors with EnableJWKSUpdate=true.
func (r *TrustAnchorRepo) AllWithJWKSUpdate() oidfed.TrustAnchors {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out oidfed.TrustAnchors
	for _, ta := range r.anchors {
		if ta.EnableJWKSUpdate {
			out = append(out, ta)
		}
	}
	return out
}

// Add adds or replaces a trust anchor in the in-memory map from a model row.
// This does not write to the database; use the storage layer for persistence.
func (r *TrustAnchorRepo) Add(m *model.TrustAnchor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.anchors[m.EntityID] = r.taFromModel(m)
}

// UpdateJWKS updates the in-memory JWKS for an entity atomically.
func (r *TrustAnchorRepo) UpdateJWKS(entityID string, jwks jwx.JWKS) {
	r.mu.RLock()
	ta, ok := r.anchors[entityID]
	r.mu.RUnlock()
	if !ok {
		return
	}
	ta.SetJWKS(jwks)
}

// Remove removes a trust anchor from the in-memory map.
func (r *TrustAnchorRepo) Remove(entityID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.anchors, entityID)
}

// AddOrUpdate loads a TA from the store by entity_id and adds or updates it in
// the in-memory repo. This is used by the admin API after a DB mutation.
func (r *TrustAnchorRepo) AddOrUpdate(entityID string) {
	item, err := r.store.Get(entityID)
	if err != nil {
		// Not found or error — remove from repo if it was there.
		r.Remove(entityID)
		return
	}
	r.Add(item)
}

// Has reports whether an entity is in the repository.
func (r *TrustAnchorRepo) Has(entityID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.anchors[entityID]
	return ok
}

// Count returns the number of trust anchors in the repository.
func (r *TrustAnchorRepo) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.anchors)
}

// Store returns the underlying trust anchor store (for admin API use).
func (r *TrustAnchorRepo) Store() model.TrustAnchorStore {
	return r.store
}

// SetupTAJWKSRefresher builds and starts the TA JWKS refresher from the
// repository. It creates a DBJWKStorage, collects all TAs with
// EnableJWKSUpdate=true from the repo, constructs a TAJWKSRefresher, and starts
// it. The returned refresher must be stopped on shutdown.
func SetupTAJWKSRefresher(repo *TrustAnchorRepo, jwkStorage oidfed.JWKStorage) (
	*oidfed.TAJWKSRefresher, error,
) {
	tas := repo.AllWithJWKSUpdate()
	refresher, err := oidfed.NewTAJWKSRefresher(&tas, jwkStorage)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create TA JWKS refresher")
	}
	if err := refresher.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start TA JWKS refresher")
	}
	log.WithField("count", len(tas)).Info("TA JWKS refresher started")
	return refresher, nil
}
