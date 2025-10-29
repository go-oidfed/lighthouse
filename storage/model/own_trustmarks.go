package model

import (
	"gorm.io/gorm"
)

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
