package model

import (
	"gorm.io/gorm"
)

// PublishedTrustMark represents a trust mark published in the entity configuration (for this entity).
type PublishedTrustMark struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt int            `json:"created_at"`
	UpdatedAt int            `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Core fields
	TrustMarkType   string `gorm:"uniqueIndex" json:"trust_mark_type"`
	TrustMarkIssuer string `json:"trust_mark_issuer,omitempty"`
	TrustMarkJWT    string `gorm:"type:text" json:"trust_mark,omitempty"`

	// Refresh configuration
	Refresh            bool `json:"refresh"`
	MinLifetime        int  `json:"min_lifetime,omitempty"`
	RefreshGracePeriod int  `json:"refresh_grace_period,omitempty"`
	RefreshRateLimit   int  `json:"refresh_rate_limit,omitempty"`

	// Self-issuance specification (stored as JSON)
	SelfIssuanceSpec *SelfIssuedTrustMarkSpec `gorm:"serializer:json" json:"self_issuance_spec,omitempty"`
}

// SelfIssuedTrustMarkSpec contains the specification for a self-issued trust mark.
type SelfIssuedTrustMarkSpec struct {
	Lifetime                 int            `json:"lifetime,omitempty"`
	Ref                      string         `json:"ref,omitempty"`
	LogoURI                  string         `json:"logo_uri,omitempty"`
	AdditionalClaims         map[string]any `json:"additional_claims,omitempty"`
	IncludeExtraClaimsInInfo bool           `json:"include_extra_claims_in_info,omitempty"`
}

// AddTrustMark is the request body for creating a trust mark.
// Provide one of:
// (1) trust_mark_type and trust_mark_issuer for external fetching,
// (2) trust_mark JWT directly (type/issuer extracted from JWT if not provided), or
// (3) self_issuance_spec for self-issued trust marks.
type AddTrustMark struct {
	TrustMarkType      string                   `json:"trust_mark_type,omitempty"`
	TrustMarkIssuer    string                   `json:"trust_mark_issuer,omitempty"`
	TrustMark          string                   `json:"trust_mark,omitempty"`
	Refresh            bool                     `json:"refresh"`
	MinLifetime        int                      `json:"min_lifetime,omitempty"`
	RefreshGracePeriod int                      `json:"refresh_grace_period,omitempty"`
	RefreshRateLimit   int                      `json:"refresh_rate_limit,omitempty"`
	SelfIssuanceSpec   *SelfIssuedTrustMarkSpec `json:"self_issuance_spec,omitempty"`
}

// UpdateTrustMark is the request body for partial updates (PATCH).
// Only non-nil fields will be updated.
type UpdateTrustMark struct {
	TrustMarkIssuer    *string                  `json:"trust_mark_issuer,omitempty"`
	TrustMark          *string                  `json:"trust_mark,omitempty"`
	Refresh            *bool                    `json:"refresh,omitempty"`
	MinLifetime        *int                     `json:"min_lifetime,omitempty"`
	RefreshGracePeriod *int                     `json:"refresh_grace_period,omitempty"`
	RefreshRateLimit   *int                     `json:"refresh_rate_limit,omitempty"`
	SelfIssuanceSpec   *SelfIssuedTrustMarkSpec `json:"self_issuance_spec,omitempty"`
}

// PublishedTrustMarksStore is the abstraction used by handlers for managing
// trust marks in the entity configuration.
type PublishedTrustMarksStore interface {
	// List returns all published trust marks.
	List() ([]PublishedTrustMark, error)
	// Create creates a new trust mark entry.
	Create(add AddTrustMark) (*PublishedTrustMark, error)
	// Get retrieves a trust mark by ID or trust_mark_type.
	Get(ident string) (*PublishedTrustMark, error)
	// Update replaces a trust mark entry entirely.
	Update(ident string, update AddTrustMark) (*PublishedTrustMark, error)
	// Patch partially updates a trust mark entry (only non-nil fields).
	Patch(ident string, patch UpdateTrustMark) (*PublishedTrustMark, error)
	// Delete removes a trust mark entry.
	Delete(ident string) error
}
