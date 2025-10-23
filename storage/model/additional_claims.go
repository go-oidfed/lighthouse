package model

import (
	"gorm.io/gorm"
)

// SubordinateAdditionalClaim stores one additional claim for a subordinate.
// value is stored as JSON; claim name is indexed; crit marks if claim is critical.
type SubordinateAdditionalClaim struct {
	gorm.Model
	SubordinateID uint   `gorm:"index"`
	Claim         string `gorm:"index"`
	Value         any    `gorm:"serializer:json"`
	Crit          bool   `gorm:"index"`
}

// EntityConfigurationAdditionalClaim stores one additional claim for the entity configuration.
type EntityConfigurationAdditionalClaim struct {
	gorm.Model
	Claim string `gorm:"index,unique"`
	Value any    `gorm:"serializer:json"`
	Crit  bool   `gorm:"index"`
}
