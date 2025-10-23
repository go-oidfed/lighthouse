package model

import (
	"time"

	"gorm.io/gorm"
)

// TrustMarkType represents a trust mark type in the database
type TrustMarkType struct {
	gorm.Model
	TrustMarkType string `gorm:"uniqueIndex"`
	OwnerID       *uint
	Owner         *TrustMarkOwner
	Description   string `gorm:"type:text"`
}

// TrustMarkOwner represents the owner of a trust mark type.
// Contains the owner's Entity ID and JWKS.
type TrustMarkOwner struct {
	gorm.Model
	EntityID    string `gorm:"uniqueIndex"`
	JWKSID      uint
	JWKS        JWKS
	Description string `gorm:"type:text"`
}

// TrustMarkTypeIssuer represents an authorized issuer for a trust mark type.
type TrustMarkTypeIssuer struct {
	gorm.Model
	TrustMarkTypeID uint `gorm:"index:,unique,composite:tmtype_issuer"`
	TrustMarkType   TrustMarkType
	Issuer          string `gorm:"index:,unique,composite:tmtype_issuer"`
}

// TrustMarkSpec represents the issuance specification for a trust mark type.
type TrustMarkSpec struct {
	gorm.Model
	TrustMarkType    string `gorm:"uniqueIndex"`
	Lifetime         uint
	Ref              string
	LogoURI          string
	DelegationJWT    string         `gorm:"type:text"`
	AdditionalClaims map[string]any `gorm:"serializer:json"`
	Description      string         `gorm:"type:text"`
}

// TrustMarkSubject represents a subject eligible for a specific trust mark issuance.
type TrustMarkSubject struct {
	gorm.Model
	TrustMarkSpecID  uint `gorm:"index:,unique,composite:tmspec_subject"`
	TrustMarkSpec    TrustMarkSpec
	EntityID         string         `gorm:"index:unique,composite:tmspec_subject"`
	Status           Status         `gorm:"index"`
	AdditionalClaims map[string]any `gorm:"serializer:json"`
	Description      string         `gorm:"type:text"`
}

// IssuedTrustMarkInstance represents an instance of a TrustMark in the database.
type IssuedTrustMarkInstance struct {
	CreatedAt          time.Time
	ExpiresAt          time.Time `gorm:"index"`
	Revoked            bool      `gorm:"index"`
	JTI                string    `gorm:"primaryKey"`
	TrustMarkSubjectID uint
	TrustMarkSubject   TrustMarkSubject
}

// PublishedTrustMark represents a trust mark published in the entity configuration (for this entity).
type PublishedTrustMark struct {
	gorm.Model
	TrustMarkType   string `gorm:"index"`
	TrustMarkIssuer string `gorm:"index"`
	TrustMarkJWT    string `gorm:"type:text"`
}
