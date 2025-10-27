package model

import (
	"gorm.io/gorm"
)

// SubordinateAdditionalClaim stores one additional claim for a subordinate.
// value is stored as JSON; claim name is indexed; crit marks if claim is critical.
type SubordinateAdditionalClaim struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     int            `json:"created_at"`
	UpdatedAt     int            `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	SubordinateID uint           `gorm:"index" json:"subordinate_id"`
	Claim         string         `gorm:"uniqueIndex" json:"claim"`
	Value         any            `gorm:"serializer:json" json:"value"`
	Crit          bool           `gorm:"index" json:"crit"`
}

// EntityConfigurationAdditionalClaim stores one additional claim for the entity configuration.
type EntityConfigurationAdditionalClaim struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt int            `json:"created_at"`
	UpdatedAt int            `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Claim     string         `gorm:"uniqueIndex" json:"claim"`
	Value     any            `gorm:"serializer:json" json:"value"`
	Crit      bool           `gorm:"index" json:"crit"`
}

// AddAdditionalClaim is a request to add an additional claim to a subordinate or entity configuration.
type AddAdditionalClaim struct {
	Claim string `json:"claim"`
	Value any    `json:"value"`
	Crit  bool   `json:"crit"`
}

// AdditionalClaimsStore abstracts CRUD for entity configuration additional claims.
type AdditionalClaimsStore interface {
	// List returns all additional claims as rows.
	List() ([]EntityConfigurationAdditionalClaim, error)
	// Set replaces the complete set of additional claims.
	Set(items []AddAdditionalClaim) ([]EntityConfigurationAdditionalClaim, error)
	// Create inserts a provided claim and returns the resulting row.
	Create(item AddAdditionalClaim) (*EntityConfigurationAdditionalClaim, error)
	// Get returns a single row by either numeric ID or claim name.
	Get(ident string) (*EntityConfigurationAdditionalClaim, error)
	// Update updates value/crit for the row identified by numeric ID or claim name.
	Update(ident string, item AddAdditionalClaim) (*EntityConfigurationAdditionalClaim, error)
	// Delete removes a single row identified by numeric ID or claim name.
	Delete(ident string) error
}
