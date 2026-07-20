package lighthouse

import (
	"strings"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// subordinateJWKSRefreshStorage adapts model.SubordinateStorageBackend to the
// oidfed.SubordinateJWKSRefreshStorage interface used by the refresher.
type subordinateJWKSRefreshStorage struct {
	store      model.SubordinateStorageBackend
	eventStore model.SubordinateEventStore
}

// NewSubordinateJWKSRefreshStorage creates an adapter over the subordinate
// storage backend that also records JWKSRefreshed events when the JWKS change.
func NewSubordinateJWKSRefreshStorage(
	store model.SubordinateStorageBackend, eventStore model.SubordinateEventStore,
) oidfed.SubordinateJWKSRefreshStorage {
	return &subordinateJWKSRefreshStorage{
		store:      store,
		eventStore: eventStore,
	}
}

func (a *subordinateJWKSRefreshStorage) ListEnabled() ([]oidfed.SubordinateJWKSInfo, error) {
	infos, err := a.store.ListEnabledForJWKSRefresh()
	if err != nil {
		return nil, err
	}
	out := make([]oidfed.SubordinateJWKSInfo, 0, len(infos))
	for i := range infos {
		info := &infos[i]
		out = append(
			out, oidfed.SubordinateJWKSInfo{
				EntityID:         info.EntityID,
				EnableJWKSUpdate: info.EnableJWKSUpdate,
				JWKSPollInterval: info.JWKSPollInterval,
				JWKS:             info.JWKS.Keys,
			},
		)
	}
	return out, nil
}

func (a *subordinateJWKSRefreshStorage) Get(entityID string) (*oidfed.SubordinateJWKSInfo, error) {
	info, err := a.store.Get(entityID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}
	return &oidfed.SubordinateJWKSInfo{
		EntityID:         info.EntityID,
		EnableJWKSUpdate: info.EnableJWKSUpdate,
		JWKSPollInterval: info.JWKSPollInterval,
		JWKS:             info.JWKS.Keys,
	}, nil
}

func (a *subordinateJWKSRefreshStorage) UpdateJWKS(entityID string, jwks jwx.JWKS) error {
	if err := a.store.UpdateJWKSByEntityID(entityID, model.NewJWKS(jwks)); err != nil {
		return err
	}
	if a.eventStore != nil {
		info, err := a.store.Get(entityID)
		if err != nil || info == nil {
			return err
		}
		if err := a.eventStore.Add(
			model.SubordinateEvent{
				SubordinateID: info.ID,
				Timestamp:     nowUnix(),
				Type:          model.EventTypeJWKSRefreshed,
			},
		); err != nil {
			log.Warn().Err(err).Str("entity_id", entityID).Msg("failed to record jwks_refreshed event")
		}
	}
	return nil
}

// RefreshSubordinateJWKSFromEC fetches the subordinate's Entity Configuration
// and updates the stored JWKS if it changed. It returns whether the JWKS changed.
//
// The subordinate must exist and have a status of Active or Pending (not
// Blocked or Inactive). The EC signature is verified against the currently
// stored JWKS.
func (fed *LightHouse) RefreshSubordinateJWKSFromEC(entityID string) (changed bool, err error) {
	store := fed.storages.Subordinates
	if store == nil {
		return false, errors.New("subordinate storage not available")
	}
	info, err := store.Get(entityID)
	if err != nil {
		return false, err
	}
	if info == nil {
		return false, errors.Errorf("subordinate %s not found", entityID)
	}
	if info.Status == model.StatusBlocked {
		return false, errors.Errorf("subordinate %s is blocked", entityID)
	}
	if info.Status == model.StatusInactive {
		return false, errors.Errorf("subordinate %s is inactive", entityID)
	}

	ec, err := oidfed.GetEntityConfiguration(entityID)
	if err != nil {
		return false, errors.Wrap(err, "failed to fetch entity configuration")
	}
	if ec.ExpiresAt.IsZero() || ec.ExpiresAt.Unix() == 0 {
		return false, errors.New("entity configuration has no exp; exp is required")
	}

	// Verify the EC signature against the currently stored JWKS.
	if info.JWKS.Keys.Set != nil && info.JWKS.Keys.Len() > 0 {
		if !ec.Verify(info.JWKS.Keys) {
			return false, errors.New("entity configuration signature verification failed against stored JWKS")
		}
	}

	oldKIDs := oidfed.ExtractKIDs(info.JWKS.Keys)
	newKIDs := oidfed.ExtractKIDs(ec.JWKS)
	changed, added, removed := oidfed.HasJWKSChanged(oldKIDs, newKIDs)
	if !changed {
		return false, nil
	}

	if err := store.UpdateJWKSByEntityID(entityID, model.NewJWKS(ec.JWKS)); err != nil {
		return false, errors.Wrap(err, "failed to update stored JWKS")
	}
	if fed.storages.SubordinateEvents != nil {
		if err := fed.storages.SubordinateEvents.Add(
			model.SubordinateEvent{
				SubordinateID: info.ID,
				Timestamp:     nowUnix(),
				Type:          model.EventTypeJWKSRefreshed,
				Message:       strPtrOrNil(addedRemovedMsg(added, removed)),
			},
		); err != nil {
			log.Warn().Err(err).Str("entity_id", entityID).Msg("failed to record jwks_refreshed event")
		}
	}
	log.Info().
		Str("entity_id", entityID).
		Strs("added", added).
		Strs("removed", removed).
		Msg("subordinate JWKS refreshed from EC")
	return true, nil
}

// notifySubordinateJWKSRefresher tells the subordinate JWKS refresher to
// reconcile its polling set for the given entity. No-op if the refresher is
// not running.
func (fed *LightHouse) notifySubordinateJWKSRefresher(entityID string) {
	if fed.subordinateJWKSRefresher == nil {
		return
	}
	if err := fed.subordinateJWKSRefresher.Update(entityID); err != nil {
		log.Debug().Err(err).Str("entity_id", entityID).
			Msg("subordinate JWKS refresher update notification failed")
	}
}

func nowUnix() int64 { return time.Now().Unix() }

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func addedRemovedMsg(added, removed []string) string {
	var parts []string
	if len(added) > 0 {
		parts = append(parts, "added: "+strings.Join(added, ", "))
	}
	if len(removed) > 0 {
		parts = append(parts, "removed: "+strings.Join(removed, ", "))
	}
	return strings.Join(parts, "; ")
}

// SetupSubordinateJWKSRefresher builds and starts the subordinate JWKS
// refresher from storage. The returned refresher must be stopped on shutdown.
func SetupSubordinateJWKSRefresher(
	store model.SubordinateStorageBackend, eventStore model.SubordinateEventStore,
) (*oidfed.SubordinateJWKSRefresher, error) {
	adapter := NewSubordinateJWKSRefreshStorage(store, eventStore)
	refresher, err := oidfed.NewSubordinateJWKSRefresher(adapter, oidfed.GetEntityConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create subordinate JWKS refresher")
	}
	if err := refresher.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start subordinate JWKS refresher")
	}
	enabled, _ := adapter.ListEnabled()
	log.Info().Int("count", len(enabled)).Msg("subordinate JWKS refresher started")
	return refresher, nil
}
