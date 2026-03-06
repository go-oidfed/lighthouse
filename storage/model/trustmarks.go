package model

import (
	"gorm.io/gorm"
)

// EligibilityMode defines how trust mark eligibility is determined
type EligibilityMode string

const (
	// EligibilityModeDBOnly checks only the TrustMarkSubject database (default)
	EligibilityModeDBOnly EligibilityMode = "db_only"
	// EligibilityModeCheckOnly runs only the entity checker, ignores DB
	EligibilityModeCheckOnly EligibilityMode = "check_only"
	// EligibilityModeDBOrCheck checks DB first, falls back to entity checker
	EligibilityModeDBOrCheck EligibilityMode = "db_or_check"
	// EligibilityModeDBAndCheck requires both DB active status AND checker pass
	EligibilityModeDBAndCheck EligibilityMode = "db_and_check"
	// EligibilityModeCustom uses a fully custom checker configuration
	EligibilityModeCustom EligibilityMode = "custom"
)

// CheckerConfig is the JSON representation of an EntityChecker configuration
type CheckerConfig struct {
	Type   string         `json:"type" yaml:"type"`
	Config map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
}

// EligibilityConfig configures how trust mark eligibility is determined
type EligibilityConfig struct {
	// Mode determines the eligibility check strategy
	Mode EligibilityMode `json:"mode" yaml:"mode"`
	// Checker is the entity checker configuration (used based on mode)
	Checker *CheckerConfig `json:"checker,omitempty" yaml:"checker,omitempty"`
	// CheckCacheTTL is how long to cache eligibility check results (seconds), 0 = no cache
	CheckCacheTTL int `json:"check_cache_ttl" yaml:"check_cache_ttl"`
}

// TrustMarkSpec represents the issuance specification for a trust mark type.
type TrustMarkSpec struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        int            `json:"created_at"`
	UpdatedAt        int            `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	TrustMarkType    string         `gorm:"uniqueIndex" json:"trust_mark_type"`
	Lifetime         uint           `json:"lifetime,omitempty"`
	Ref              string         `json:"ref,omitempty"`
	LogoURI          string         `json:"logo_uri,omitempty"`
	DelegationJWT    string         `gorm:"type:text" json:"delegation_jwt,omitempty"`
	AdditionalClaims map[string]any `gorm:"serializer:json" json:"additional_claims,omitempty"`
	Description      string         `gorm:"type:text" json:"description,omitempty"`
	// EligibilityConfig defines how eligibility for this trust mark is determined
	EligibilityConfig *EligibilityConfig `gorm:"serializer:json" json:"eligibility_config,omitempty"`
}

// TrustMarkSubject represents a subject eligible for a specific trust mark issuance.
type TrustMarkSubject struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        int            `json:"created_at"`
	UpdatedAt        int            `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	TrustMarkSpecID  uint           `gorm:"index:,unique,composite:tmspec_subject" json:"-"`
	TrustMarkSpec    TrustMarkSpec  `json:"-"`
	EntityID         string         `gorm:"index:unique,composite:tmspec_subject" json:"entity_id"`
	Status           Status         `gorm:"index" json:"status"`
	AdditionalClaims map[string]any `gorm:"serializer:json" json:"additional_claims,omitempty"`
	Description      string         `gorm:"type:text" json:"description,omitempty"`
}

// IssuedTrustMarkInstance represents an instance of a TrustMark in the database.
// Each record tracks a specific trust mark JWT that was issued, enabling
// revocation checking and status queries per the OIDC Federation spec.
type IssuedTrustMarkInstance struct {
	JTI                string           `gorm:"primaryKey" json:"jti"`
	CreatedAt          int              `json:"created_at"`
	UpdatedAt          int              `json:"updated_at"`
	ExpiresAt          int              `gorm:"index" json:"expires_at"`
	Revoked            bool             `gorm:"index" json:"revoked"`
	TrustMarkSubjectID uint             `gorm:"index" json:"trust_mark_subject_id"`
	TrustMarkSubject   TrustMarkSubject `json:"trust_mark_subject"`
	// TrustMarkType is denormalized for efficient lookups without joins
	TrustMarkType string `gorm:"index" json:"trust_mark_type"`
	// Subject is the entity ID that received this trust mark (denormalized)
	Subject string `gorm:"index" json:"subject"`
}

// TrustMarkInstanceStatus represents the status of an issued trust mark instance
type TrustMarkInstanceStatus string

const (
	// TrustMarkStatusActive indicates the trust mark is valid and active
	TrustMarkStatusActive TrustMarkInstanceStatus = "active"
	// TrustMarkStatusExpired indicates the trust mark has expired
	TrustMarkStatusExpired TrustMarkInstanceStatus = "expired"
	// TrustMarkStatusRevoked indicates the trust mark was revoked
	TrustMarkStatusRevoked TrustMarkInstanceStatus = "revoked"
	// TrustMarkStatusInvalid indicates the trust mark signature validation failed
	TrustMarkStatusInvalid TrustMarkInstanceStatus = "invalid"
)

// IssuedTrustMarkInstanceStore provides operations for tracking issued trust mark instances
type IssuedTrustMarkInstanceStore interface {
	// Create records a new issued trust mark instance
	Create(instance *IssuedTrustMarkInstance) error
	// GetByJTI retrieves an instance by its JTI (JWT ID)
	GetByJTI(jti string) (*IssuedTrustMarkInstance, error)
	// Revoke marks a trust mark instance as revoked
	Revoke(jti string) error
	// RevokeBySubjectID revokes all instances for a given TrustMarkSubjectID.
	// Returns the number of revoked instances.
	RevokeBySubjectID(subjectID uint) (int64, error)
	// GetStatus returns the status of a trust mark instance
	GetStatus(jti string) (TrustMarkInstanceStatus, error)
	// ListBySubject returns all instances for a given trust mark type and subject
	ListBySubject(trustMarkType, entityID string) ([]IssuedTrustMarkInstance, error)
	// ListActiveSubjects returns distinct entity IDs that have valid (non-revoked, non-expired)
	// trust marks for the given trust mark type. Used by the trust marked entities listing endpoint.
	ListActiveSubjects(trustMarkType string) ([]string, error)
	// HasActiveInstance checks if an entity has a valid (non-revoked, non-expired)
	// trust mark instance for the given trust mark type
	HasActiveInstance(trustMarkType, entityID string) (bool, error)
	// DeleteExpired removes expired instances older than the given retention period
	DeleteExpired(retentionDays int) (int64, error)
	// FindSubjectID looks up the TrustMarkSubjectID for a given trust mark type and entity
	FindSubjectID(trustMarkType, entityID string) (uint, error)
}

// TrustMarkSpecStore provides CRUD for TrustMarkSpec and TrustMarkSubject entities
type TrustMarkSpecStore interface {
	// Spec operations
	List() ([]TrustMarkSpec, error)
	Create(spec *TrustMarkSpec) (*TrustMarkSpec, error)
	Get(ident string) (*TrustMarkSpec, error)
	GetByType(trustMarkType string) (*TrustMarkSpec, error)
	Update(ident string, spec *TrustMarkSpec) (*TrustMarkSpec, error)
	Patch(ident string, updates map[string]any) (*TrustMarkSpec, error)
	Delete(ident string) error

	// Subject operations
	ListSubjects(specIdent string, status *Status) ([]TrustMarkSubject, error)
	CreateSubject(specIdent string, subject *TrustMarkSubject) (*TrustMarkSubject, error)
	GetSubject(specIdent, subjectIdent string) (*TrustMarkSubject, error)
	UpdateSubject(specIdent, subjectIdent string, subject *TrustMarkSubject) (*TrustMarkSubject, error)
	DeleteSubject(specIdent, subjectIdent string) error
	ChangeSubjectStatus(specIdent, subjectIdent string, status Status) (*TrustMarkSubject, error)
}
