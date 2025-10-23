package model

import (
	oidfed "github.com/go-oidfed/lib"
	"gorm.io/gorm"
)

// Status is a type for holding a status for something that is stored in the
// database; this type describes the status or state of the entity,
// e.g. "blocked" or "active"
type Status int

// Constants for Status
const (
	StatusActive Status = iota
	StatusBlocked
	StatusPending
	StatusInactive
)

// SubordinateInfo holds information about a subordinate for storage
// Table name is set to `subordinates` to replace legacy `subordinate_infos`.
type SubordinateInfo struct {
	ID                 uint                            `gorm:"primarykey" json:"id"`
	CreatedAt          int                             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          int                             `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt          gorm.DeletedAt                  `gorm:"index" json:"-"`
	EntityID           string                          `gorm:"uniqueIndex" json:"entity_id"`
	Description        string                          `gorm:"type:text" json:"description"`
	EntityTypes        EntityTypes                     `gorm:"many2many:subordinate_entity_types" json:"entity_types"`
	JWKSID             uint                            `json:"jwks_id"`
	JWKS               JWKS                            `json:"jwks"`
	Metadata           *oidfed.Metadata                `gorm:"serializer:json" json:"metadata,omitempty"`
	MetadataPolicy     *oidfed.MetadataPolicies        `gorm:"serializer:json" json:"metadata_policies,omitempty"`
	Constraints        *oidfed.ConstraintSpecification `gorm:"serializer:json" json:"constraints,omitempty"`
	MetadataPolicyCrit PolicyOperators                 `gorm:"many2many:subordinates_policy_operators" json:"metadata_policy_crit,omitempty"`
	Status             Status                          `gorm:"index" json:"status"`
}

func (SubordinateInfo) TableName() string { return "subordinates" }

// Removed CritExtensions and subordinate_crit_extensions per db-fixes.

// PolicyOperator represents a policy operator in the database.
type PolicyOperator struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	CreatedAt      int            `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      int            `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	PolicyOperator string         `json:"policy_operator"`
}

// PolicyOperators is a collection of PolicyOperator objects.
// This type provides methods for working with multiple policy operators together.
type PolicyOperators []PolicyOperator

// NewPolicyOperators creates a new PolicyOperators collection from a slice of
// oidfed.PolicyOperatorName.
// Each string is converted to a PolicyOperator object.
func NewPolicyOperators(operators []oidfed.PolicyOperatorName) PolicyOperators {
	policyOperators := make(PolicyOperators, len(operators))
	for i, t := range operators {
		policyOperators[i] = PolicyOperator{
			PolicyOperator: string(t),
		}
	}
	return policyOperators
}

// NewPolicyOperatorsFromStrings creates a new PolicyOperators collection from a
// slice of strings.
// Each string is converted to a PolicyOperator object.
func NewPolicyOperatorsFromStrings(operators []string) PolicyOperators {
	policyOperators := make(PolicyOperators, len(operators))
	for i, t := range operators {
		policyOperators[i] = PolicyOperator{
			PolicyOperator: t,
		}
	}
	return policyOperators
}

// ToStrings converts a PolicyOperators collection to a slice of strings.
// Each PolicyOperator is extracted into the resulting slice.
func (et PolicyOperators) ToStrings() []string {
	result := make([]string, len(et))
	for i, t := range et {
		result[i] = t.PolicyOperator
	}
	return result
}

// ToPolicyOperatorNames converts a PolicyOperators collection to a slice of
// oidfed.PolicyOperatorName.
func (et PolicyOperators) ToPolicyOperatorNames() []oidfed.PolicyOperatorName {
	result := make([]oidfed.PolicyOperatorName, len(et))
	for i, t := range et {
		result[i] = oidfed.PolicyOperatorName(t.PolicyOperator)
	}
	return result
}
