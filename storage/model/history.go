package model

import (
	"github.com/lestrrat-go/jwx/v3/jwk"
	"gorm.io/gorm"
)

// HistoricalKey represents a previously active key that has been revoked.
type HistoricalKey struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt int            `json:"created_at"`
	UpdatedAt int            `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	KID       string         `gorm:"index" json:"kid"`
	JWK       jwk.Key        `gorm:"serializer:json" json:"jwk"`
	RevokedAt int            `json:"revoked_at"`
	Reason    string         `json:"reason"`
}
