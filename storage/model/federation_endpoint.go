package model

import (
	"slices"

	"gorm.io/gorm"
)

// FederationEndpointType enumerates the federation endpoint types managed in
// the database. There is at most one endpoint of each type.
type FederationEndpointType string

const (
	EndpointTypeFetch             FederationEndpointType = "fetch"
	EndpointTypeList              FederationEndpointType = "list"
	EndpointTypeResolve           FederationEndpointType = "resolve"
	EndpointTypeTrustMark         FederationEndpointType = "trust_mark"
	EndpointTypeTrustMarkStatus   FederationEndpointType = "trust_mark_status"
	EndpointTypeTrustMarkListing  FederationEndpointType = "trust_mark_listing"
	EndpointTypeHistoricalKeys    FederationEndpointType = "historical_keys"
	EndpointTypeEnroll            FederationEndpointType = "enroll"
	EndpointTypeEnrollRequest     FederationEndpointType = "enroll_request"
	EndpointTypeTrustMarkRequest  FederationEndpointType = "trust_mark_request"
	EndpointTypeEntityCollection  FederationEndpointType = "entity_collection"
	EndpointTypeJwksUpdateTrigger FederationEndpointType = "jwks_update_trigger"
	EndpointTypeJwksUpdate        FederationEndpointType = "jwks_update"
)

// AllFederationEndpointTypes returns all valid endpoint types.
func AllFederationEndpointTypes() []FederationEndpointType {
	return []FederationEndpointType{
		EndpointTypeFetch,
		EndpointTypeList,
		EndpointTypeResolve,
		EndpointTypeTrustMark,
		EndpointTypeTrustMarkStatus,
		EndpointTypeTrustMarkListing,
		EndpointTypeHistoricalKeys,
		EndpointTypeEnroll,
		EndpointTypeEnrollRequest,
		EndpointTypeTrustMarkRequest,
		EndpointTypeEntityCollection,
		EndpointTypeJwksUpdateTrigger,
		EndpointTypeJwksUpdate,
	}
}

// IsValidFederationEndpointType reports whether t is a known endpoint type.
func IsValidFederationEndpointType(t FederationEndpointType) bool {
	return slices.Contains(AllFederationEndpointTypes(), t)
}

// FederationEndpointAuthTA is the explicit join table for the many-to-many
// relation between FederationEndpoint and TrustAnchor. The CASCADE foreign key
// constraints are declared via the constraint tag on the AuthTrustAnchors
// relationship field on FederationEndpoint, so that GORM creates named
// constraints that AutoMigrate can detect on subsequent boots.
type FederationEndpointAuthTA struct {
	FederationEndpointID uint `gorm:"primaryKey"`
	TrustAnchorID        uint `gorm:"primaryKey"`
}

// TableName overrides the default GORM table name to match the many2many tag.
func (FederationEndpointAuthTA) TableName() string {
	return "federation_endpoint_auth_trust_anchors"
}

// FederationEndpoint represents a federation endpoint managed in the database.
// Path and URL are nullable: a nil Path means the endpoint is disabled (no
// route is served and the corresponding federation metadata field is omitted).
//
// Type-specific configuration (allowed_trust_anchors, grace_period, proactive
// resolver settings, collection interval/concurrency/pagination, enroll entity
// checker config, etc.) is stored in the Config JSON blob and interpreted by
// the type-specific handler factory.
//
// AuthTrustAnchors is a many-to-many relation to TrustAnchor via the
// federation_endpoint_auth_trust_anchors join table.
type FederationEndpoint struct {
	ID               uint                   `gorm:"primarykey" json:"id"`
	CreatedAt        int                    `json:"created_at"`
	UpdatedAt        int                    `json:"updated_at"`
	DeletedAt        gorm.DeletedAt         `gorm:"index" json:"-"`
	Type             FederationEndpointType `gorm:"size:64;uniqueIndex" json:"type"`
	Path             *string                `gorm:"size:512;uniqueIndex" json:"path,omitempty"`
	URL              *string                `gorm:"size:1024" json:"url,omitempty"`
	AuthEnabled      bool                   `json:"auth_enabled"`
	Config           string                 `gorm:"type:text" json:"config,omitempty"`
	AuthTrustAnchors []TrustAnchor          `gorm:"many2many:federation_endpoint_auth_trust_anchors;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"auth_trust_anchors,omitempty"`
}

// FederationEndpointStore is the storage interface for federation endpoints.
type FederationEndpointStore interface {
	List() ([]FederationEndpoint, error)
	GetByType(t FederationEndpointType) (*FederationEndpoint, error)
	GetByPath(path string) (*FederationEndpoint, error)
	GetByID(id uint) (*FederationEndpoint, error)
	Create(req AddFederationEndpoint) (*FederationEndpoint, error)
	Update(t FederationEndpointType, req AddFederationEndpoint) (*FederationEndpoint, error)
	Delete(t FederationEndpointType) error
	SetAuthTrustAnchors(t FederationEndpointType, trustAnchorIDs []uint) ([]TrustAnchor, error)
}

// AddFederationEndpoint is the request payload to create/update a FederationEndpoint.
type AddFederationEndpoint struct {
	Type             FederationEndpointType `json:"type"`
	Path             *string                `json:"path,omitempty"`
	URL              *string                `json:"url,omitempty"`
	AuthEnabled      bool                   `json:"auth_enabled"`
	Config           string                 `json:"config,omitempty"`
	AuthTrustAnchors []uint                 `json:"auth_trust_anchor_ids,omitempty"`
}
