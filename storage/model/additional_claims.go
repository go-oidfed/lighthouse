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
