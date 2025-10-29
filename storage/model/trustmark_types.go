package model

import (
	"encoding/json"
	"strconv"

	oidfed "github.com/go-oidfed/lib"
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
	ID              uint            `gorm:"primarykey" json:"id"`
	CreatedAt       int             `json:"created_at"`
	UpdatedAt       int             `json:"updated_at"`
	DeletedAt       gorm.DeletedAt  `gorm:"index" json:"-"`
	TrustMarkTypeID uint            `gorm:"index:,unique,composite:tmtype_issuer" json:"trust_mark_type_id"`
	TrustMarkType   TrustMarkType   `json:"trust_mark_type"`
	IssuerID        uint            `gorm:"index:,unique,composite:tmtype_issuer" json:"issuer_id"`
	Issuer          TrustMarkIssuer `json:"issuer"`
}

// AddTrustMarkOwner is the request payload to create/update a TrustMarkOwner
type AddTrustMarkOwner struct {
	OwnerID  *string `json:"owner_id,omitempty"`
	EntityID string  `json:"entity_id,omitempty"`
	JWKS     JWKS    `json:"jwks,omitempty"`
}

// UnmarshalJSON allows owner_id to be provided as either a number or a numeric string.
func (a *AddTrustMarkOwner) UnmarshalJSON(data []byte) error {
	// Wire format with flexible owner_id
	var wire struct {
		OwnerID  json.RawMessage `json:"owner_id"`
		EntityID string          `json:"entity_id,omitempty"`
		JWKS     JWKS            `json:"jwks,omitempty"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	a.EntityID = wire.EntityID
	a.JWKS = wire.JWKS
	if len(wire.OwnerID) != 0 && string(wire.OwnerID) != "null" {
		var asUint uint64
		if err := json.Unmarshal(wire.OwnerID, &asUint); err == nil {
			s := strconv.FormatUint(asUint, 10)
			a.OwnerID = &s
		} else {
			var asStr string
			if err2 := json.Unmarshal(wire.OwnerID, &asStr); err2 == nil && asStr != "" {
				a.OwnerID = &asStr
			}
		}
	}
	return nil
}

// AddTrustMarkType is the request payload to create/update a TrustMarkType
// Optional fields are used by separate endpoints; issuers and owner can be set
// during creation for convenience.
type AddTrustMarkType struct {
	TrustMarkType    string               `json:"trust_mark_type"`
	Description      string               `json:"description,omitempty"`
	TrustMarkOwner   *AddTrustMarkOwner   `json:"trust_mark_owner,omitempty"`
	TrustMarkIssuers []AddTrustMarkIssuer `json:"trust_mark_issuers,omitempty"`
}

// TrustMarkTypesStore abstracts CRUD and relations for TrustMarkType, its owner, and issuers.
type TrustMarkTypesStore interface {
	// Types
	List() ([]TrustMarkType, error)
	Create(req AddTrustMarkType) (*TrustMarkType, error)
	Get(ident string) (*TrustMarkType, error)
	Update(ident string, req AddTrustMarkType) (*TrustMarkType, error)
	Delete(ident string) error
	// Aggregates
	OwnersByType() (oidfed.TrustMarkOwners, error)
	IssuersByType() (oidfed.AllowedTrustMarkIssuers, error)

	// Issuers
	ListIssuers(ident string) ([]TrustMarkIssuer, error)
	SetIssuers(ident string, issuers []AddTrustMarkIssuer) ([]TrustMarkIssuer, error)
	AddIssuer(ident string, issuer AddTrustMarkIssuer) ([]TrustMarkIssuer, error)
	DeleteIssuerByID(ident string, issuerID uint) ([]TrustMarkIssuer, error)

	// Owner
	GetOwner(ident string) (*TrustMarkOwner, error)
	CreateOwner(ident string, req AddTrustMarkOwner) (*TrustMarkOwner, error)
	UpdateOwner(ident string, req AddTrustMarkOwner) (*TrustMarkOwner, error)
	DeleteOwner(ident string) error
}

// AddTrustMarkIssuer represents the request payload to add/set a trust mark issuer
type AddTrustMarkIssuer struct {
	IssuerID    *string `json:"issuer_id,omitempty"`
	Issuer      string  `json:"issuer,omitempty"`
	Description string  `json:"description,omitempty"`
}

// UnmarshalJSON allows issuer_id to be provided as either a number or a numeric string.
func (a *AddTrustMarkIssuer) UnmarshalJSON(data []byte) error {
	var wire struct {
		IssuerID    json.RawMessage `json:"issuer_id"`
		Issuer      string          `json:"issuer,omitempty"`
		Description string          `json:"description,omitempty"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	a.Issuer = wire.Issuer
	a.Description = wire.Description
	if len(wire.IssuerID) != 0 && string(wire.IssuerID) != "null" {
		var asUint uint64
		if err := json.Unmarshal(wire.IssuerID, &asUint); err == nil {
			s := strconv.FormatUint(asUint, 10)
			a.IssuerID = &s
		} else {
			var asStr string
			if err2 := json.Unmarshal(wire.IssuerID, &asStr); err2 == nil && asStr != "" {
				a.IssuerID = &asStr
			}
		}
	}
	return nil
}

// TrustMarkIssuer represents the issuer object returned by the API
type TrustMarkIssuer struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   int            `json:"created_at"`
	UpdatedAt   int            `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Issuer      string         `gorm:"uniqueIndex" json:"issuer"`
	Description string         `gorm:"type:text" json:"description"`
}

// TrustMarkOwnersStore manages global owners and their relations to types.
type TrustMarkOwnersStore interface {
	List() ([]TrustMarkOwner, error)
	Create(req AddTrustMarkOwner) (*TrustMarkOwner, error)
	Get(ident string) (*TrustMarkOwner, error)
	Update(ident string, req AddTrustMarkOwner) (*TrustMarkOwner, error)
	Delete(ident string) error
	// Relations
	Types(ident string) ([]uint, error)
	SetTypes(ident string, typeIdents []string) ([]uint, error)
	AddType(ident string, typeID uint) ([]uint, error)
	DeleteType(ident string, typeID uint) ([]uint, error)
}

// TrustMarkIssuersStore manages global issuers and their relations to types.
type TrustMarkIssuersStore interface {
	List() ([]TrustMarkIssuer, error)
	Create(req AddTrustMarkIssuer) (*TrustMarkIssuer, error)
	Get(ident string) (*TrustMarkIssuer, error)
	Update(ident string, req AddTrustMarkIssuer) (*TrustMarkIssuer, error)
	Delete(ident string) error
	// Relations
	Types(ident string) ([]uint, error)
	SetTypes(ident string, typeIdents []string) ([]uint, error)
	AddType(ident string, typeID uint) ([]uint, error)
	DeleteType(ident string, typeID uint) ([]uint, error)
}
