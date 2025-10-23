package model

import (
	"github.com/lestrrat-go/jwx/v3/jwk"
	"gorm.io/gorm"
	"time"
)

// HistoricalKey represents a previously active key that has been revoked.
type HistoricalKey struct {
	gorm.Model
	KID       string  `gorm:"index"`
	JWK       jwk.Key `gorm:"serializer:json"`
	RevokedAt time.Time
	Reason    string
}
