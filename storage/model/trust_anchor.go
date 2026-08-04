package model

import (
	"gorm.io/gorm"

	"github.com/go-oidfed/lib/jwx"
)

// TrustAnchor represents a trust anchor managed in the database, with its JWKS
// stored as a has-one relation to the JWKS table (same pattern as TrustMarkOwner
// and ExtendedSubordinateInfo).
//
// This is the common TA repository: all usages of a TA across LightHouse
// (client auth, trust marks, entity checks, ...) reference a row here by
// entity_id so JWKS is stored and refreshed in a single place.
type TrustAnchor struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        int            `json:"created_at"`
	UpdatedAt        int            `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	EntityID         string         `gorm:"size:512;uniqueIndex" json:"entity_id"`
	JWKSID           *uint          `json:"jwks_id,omitempty"`
	JWKS             JWKS           `json:"jwks"`
	EnableJWKSUpdate bool           `json:"enable_jwks_update"`
	KeyPollInterval  int64          `json:"key_poll_interval"` // seconds; 0 = derive from EC expiration
}

// AddTrustAnchor is the request payload to create/update a TrustAnchor
type AddTrustAnchor struct {
	EntityID         string `json:"entity_id"`
	JWKS             *JWKS  `json:"jwks,omitempty"`
	EnableJWKSUpdate bool   `json:"enable_jwks_update"`
	KeyPollInterval  int64  `json:"key_poll_interval"`
}

// TrustAnchorStore is the storage interface for trust anchors
type TrustAnchorStore interface {
	List() ([]TrustAnchor, error)
	Get(entityID string) (*TrustAnchor, error)
	GetByID(id uint) (*TrustAnchor, error)
	Create(req AddTrustAnchor) (*TrustAnchor, error)
	Update(entityID string, req AddTrustAnchor) (*TrustAnchor, error)
	Delete(entityID string) error
	// GetJWKS returns the stored JWKS for an entity, or nil if none stored.
	GetJWKS(entityID string) (*jwx.JWKS, error)
	// UpdateJWKS stores/replaces the JWKS for an entity. If the entity has no
	// JWKS row yet, one is created and linked.
	UpdateJWKS(entityID string, jwks jwx.JWKS) error
}
