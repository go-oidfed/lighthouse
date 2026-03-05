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
