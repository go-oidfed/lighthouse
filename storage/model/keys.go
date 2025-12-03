package model

import (
	"github.com/go-oidfed/lib/jwx"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"gorm.io/gorm"
)

// Key represents a single jwk.Key in the database
type Key struct {
	KID       string  `gorm:"primaryKey;column:kid" json:"kid"`
	CreatedAt int     `json:"created_at"`
	UpdatedAt int     `json:"updated_at"`
	JWK       jwk.Key `gorm:"serializer:json" json:"jwk"`
}

// JWKS represents a set of Key, i.e. a jwk.Set in the database
type JWKS struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt int            `json:"created_at"`
	UpdatedAt int            `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Keys      []Key          `gorm:"many2many:jwks_keys" json:"keys"`
}

// JWKS returns the keys as a jwx.JWKS
func (jwks JWKS) JWKS() jwx.JWKS {
	set := jwx.NewJWKS()
	for _, k := range jwks.Keys {
		_ = set.AddKey(k.JWK)
	}
	return set
}

func NewJWKS(jwks jwx.JWKS) JWKS {
	l := jwks.Len()
	keys := make([]Key, l)
	for i := 0; i < l; i++ {
		k, _ := jwks.Key(i)
		keyID, ok := k.KeyID()
		if !ok {
			_ = jwk.AssignKeyID(k)
			keyID, _ = k.KeyID()
		}
		keys[i] = Key{
			KID: keyID,
			JWK: k,
		}
	}
	return JWKS{
		Keys: keys,
	}
}
