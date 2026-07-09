package storage

import (
	"github.com/pkg/errors"

	"github.com/go-oidfed/lib/jwx"
)

// DBJWKStorage implements the oidfed.JWKStorage interface backed by the
// lighthouse TrustAnchorStorage. It stores and retrieves JWKS for trust
// anchors in the database, reusing the existing model.JWKS table (no new
// table is introduced).
//
// RegisterEntityJWKSFile is a no-op since lighthouse TAs store their JWKS in
// the database rather than in files.
type DBJWKStorage struct {
	store *TrustAnchorStorage
}

// NewDBJWKStorage creates a new DBJWKStorage wrapping the given TrustAnchorStorage.
func NewDBJWKStorage(store *TrustAnchorStorage) *DBJWKStorage {
	return &DBJWKStorage{store: store}
}

// UpdateJWKS stores the complete JWKS for an entity, replacing any existing
// JWKS for this entityID.
func (d *DBJWKStorage) UpdateJWKS(entityID string, jwks jwx.JWKS) error {
	if d == nil || d.store == nil {
		return errors.New("db jwk storage is not initialized")
	}
	return d.store.UpdateJWKS(entityID, jwks)
}

// GetJWKS retrieves the stored JWKS for an entity.
// Returns nil, nil if no JWKS is stored for the entityID.
func (d *DBJWKStorage) GetJWKS(entityID string) (*jwx.JWKS, error) {
	if d == nil || d.store == nil {
		return nil, nil
	}
	return d.store.GetJWKS(entityID)
}

// RegisterEntityJWKSFile is a no-op for lighthouse: JWKS are stored in the
// database, not in files. It satisfies the oidfed.JWKStorage interface.
func (*DBJWKStorage) RegisterEntityJWKSFile(_, _ string) error {
	return nil
}
