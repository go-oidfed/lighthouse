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
type SubordinateInfo struct {
	gorm.Model
	EntityDBID         uint   `gorm:"uniqueIndex"`
	Entity             Entity `gorm:"foreignKey:EntityDBID"`
	JWKSID             uint
	JWKS               JWKS                            `json:"jwks"`
	Metadata           *oidfed.Metadata                `json:"metadata,omitempty" gorm:"serializer:json"`
	MetadataPolicy     *oidfed.MetadataPolicies        `json:"metadata_policy,omitempty" gorm:"serializer:json"`
	Constraints        *oidfed.ConstraintSpecification `json:"constraints,omitempty" gorm:"serializer:json"`
	CriticalExtensions CritExtensions                  `json:"crit,omitempty" gorm:"many2many:subordinates_crit_extensions"`
	MetadataPolicyCrit PolicyOperators                 `json:"metadata_policy_crit,omitempty" gorm:"many2many:subordinates_policy_operators"`
	Extra              map[string]interface{}          `json:"-" gorm:"serializer:json"`
	Status             Status                          `json:"status" gorm:"index"`
}

// CritExtension represents a critical extension in the database
type CritExtension struct {
	gorm.Model
	CritExtension string
}

// CritExtensions is a collection of CritExtension objects.
// This type provides methods for working with multiple crit extensions
// together.
type CritExtensions []CritExtension

// NewCritExtensions creates a new CritExtensions collection from a slice of
// strings. Each string is converted to an CritExtension object.
func NewCritExtensions(extensions []string) CritExtensions {
	critExtensions := make(CritExtensions, len(extensions))
	for i, t := range extensions {
		critExtensions[i] = CritExtension{
			CritExtension: t,
		}
	}
	return critExtensions
}

// ToStrings converts a CritExtensions collection to a slice of strings.
// Each CritExtension  is extracted into the resulting slice.
func (et CritExtensions) ToStrings() []string {
	result := make([]string, len(et))
	for i, t := range et {
		result[i] = t.CritExtension
	}
	return result
}

// PolicyOperator represents a policy operator in the database.
type PolicyOperator struct {
	gorm.Model
	PolicyOperator string
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
