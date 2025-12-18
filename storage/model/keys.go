package model

import (
	"encoding/json"

	"github.com/go-oidfed/lib/jwx"
	"gorm.io/gorm"
)

// JWKS represents a set of Key, i.e. a jwk.Set in the database
type JWKS struct {
	ID        uint           `gorm:"primarykey"`
	CreatedAt int            `json:"created_at"`
	UpdatedAt int            `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Keys      jwx.JWKS       `gorm:"serializer:json"`
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (j *JWKS) UnmarshalJSON(bytes []byte) error {
	return json.Unmarshal(bytes, &j.Keys)
}

// MarshalJSON implements the json.Marshaler interface.
func (j JWKS) MarshalJSON() ([]byte, error) {
	if j.Keys.Set == nil {
		return []byte("null"), nil
	}
	k, err := j.Keys.Clone()
	if err != nil {
		return nil, err
	}
	// k := j.Keys.Set
	_ = k.Set("id", j.ID)
	return json.Marshal(k)
}

func NewJWKS(jwks jwx.JWKS) JWKS {
	return JWKS{
		Keys: jwks,
	}
}
