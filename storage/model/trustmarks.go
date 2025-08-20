package model

import (
	"time"

	"gorm.io/gorm"
)

// TrustMarkType represents a trust mark type in the database
type TrustMarkType struct {
	gorm.Model
	TrustMarkType string `gorm:"uniqueIndex"`
}

// TrustMarkedEntity represents a trust marked entity in the database,
// mapping an entity with a TrustMarkType
type TrustMarkedEntity struct {
	gorm.Model
	TrustMarkTypeID uint `gorm:"index:,unique,composite:trustmarkedentity"`
	TrustMarkType   TrustMarkType
	EntityDBID      uint   `gorm:"index:,unique,composite:trustmarkedentity"`
	Entity          Entity `gorm:"foreignKey:EntityDBID"`
	Status          Status `gorm:"index"`
}

// TrustMarkInstance represents an instance of a TrustMark in the database.
type TrustMarkInstance struct {
	CreatedAt           time.Time
	ExpiresAt           time.Time `gorm:"index"`
	Revoked             bool      `gorm:"index"`
	JTI                 string    `gorm:"primaryKey"`
	TrustMarkedEntityID uint
	TrustMarkedEntity   TrustMarkedEntity
}
