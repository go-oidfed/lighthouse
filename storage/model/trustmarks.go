package model

import (
	"gorm.io/gorm"
)

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
