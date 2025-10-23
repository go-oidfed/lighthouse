package model

import (
	"gorm.io/gorm"
)

// TrustMarkType represents a trust mark type in the database
type TrustMarkType struct {
	ID            uint            `gorm:"primarykey" json:"id"`
	CreatedAt     int             `json:"created_at"`
	UpdatedAt     int             `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"-"`
	TrustMarkType string          `gorm:"uniqueIndex" json:"trust_mark_type"`
	OwnerID       *uint           `json:"owner_id,omitempty"`
	Owner         *TrustMarkOwner `json:"owner,omitempty"`
	Description   string          `gorm:"type:text" json:"description"`
}

// TrustMarkOwner represents the owner of a trust mark type.
// Contains the owner's Entity ID and JWKS.
type TrustMarkOwner struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   int            `json:"created_at"`
	UpdatedAt   int            `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	EntityID    string         `gorm:"uniqueIndex" json:"entity_id"`
	JWKSID      uint           `json:"jwks_id"`
	JWKS        JWKS           `json:"jwks"`
	Description string         `gorm:"type:text" json:"description"`
}

// TrustMarkTypeIssuer represents an authorized issuer for a trust mark type.
type TrustMarkTypeIssuer struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt       int            `json:"created_at"`
	UpdatedAt       int            `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	TrustMarkTypeID uint           `gorm:"index:,unique,composite:tmtype_issuer" json:"trust_mark_type_id"`
	TrustMarkType   TrustMarkType  `json:"trust_mark_type"`
	Issuer          string         `gorm:"index:,unique,composite:tmtype_issuer" json:"issuer"`
}

// TrustMarkSpec represents the issuance specification for a trust mark type.
type TrustMarkSpec struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        int            `json:"created_at"`
	UpdatedAt        int            `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	TrustMarkType    string         `gorm:"uniqueIndex" json:"trust_mark_type"`
	Lifetime         uint           `json:"lifetime"`
	Ref              string         `json:"ref"`
	LogoURI          string         `json:"logo_uri"`
	DelegationJWT    string         `gorm:"type:text" json:"delegation_jwt"`
	AdditionalClaims map[string]any `gorm:"serializer:json" json:"additional_claims"`
	Description      string         `gorm:"type:text" json:"description"`
}

// TrustMarkSubject represents a subject eligible for a specific trust mark issuance.
type TrustMarkSubject struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        int            `json:"created_at"`
	UpdatedAt        int            `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	TrustMarkSpecID  uint           `gorm:"index:,unique,composite:tmspec_subject" json:"trust_mark_spec_id"`
	TrustMarkSpec    TrustMarkSpec  `json:"trust_mark_spec"`
	EntityID         string         `gorm:"index:unique,composite:tmspec_subject" json:"entity_id"`
	Status           Status         `gorm:"index" json:"status"`
	AdditionalClaims map[string]any `gorm:"serializer:json" json:"additional_claims"`
	Description      string         `gorm:"type:text" json:"description"`
}

// IssuedTrustMarkInstance represents an instance of a TrustMark in the database.
type IssuedTrustMarkInstance struct {
	JTI                string           `gorm:"primaryKey" json:"jti"`
	CreatedAt          int              `json:"created_at"`
	UpdatedAt          int              `json:"updated_at"`
	ExpiresAt          int              `gorm:"index" json:"expires_at"`
	Revoked            bool             `gorm:"index" json:"revoked"`
	TrustMarkSubjectID uint             `json:"trust_mark_subject_id"`
	TrustMarkSubject   TrustMarkSubject `json:"trust_mark_subject"`
}

// PublishedTrustMark represents a trust mark published in the entity configuration (for this entity).
type PublishedTrustMark struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt       int            `json:"created_at"`
	UpdatedAt       int            `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	TrustMarkType   string         `gorm:"index" json:"trust_mark_type"`
	TrustMarkIssuer string         `gorm:"index" json:"trust_mark_issuer"`
	TrustMarkJWT    string         `gorm:"type:text" json:"trust_mark"`
}
