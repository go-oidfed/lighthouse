package model

import (
	"encoding/json"

	oidfed "github.com/go-oidfed/lib"
	"gorm.io/gorm"
)

// ExtendedSubordinateInfo holds information about a subordinate for storage
// Table name is set to `subordinates` to replace legacy `subordinate_infos`.
type ExtendedSubordinateInfo struct {
	BasicSubordinateInfo
	JWKSID                      uint                            `json:"-"`
	JWKS                        JWKS                            `json:"jwks"`
	Metadata                    *oidfed.Metadata                `gorm:"serializer:json" json:"metadata,omitempty"`
	MetadataPolicy              *oidfed.MetadataPolicies        `gorm:"serializer:json" json:"metadata_policy,omitempty"`
	Constraints                 *oidfed.ConstraintSpecification `gorm:"serializer:json" json:"constraints,omitempty"`
	MetadataPolicyCrit          PolicyOperators                 `gorm:"many2many:subordinates_policy_operators" json:"metadata_policy_crit,omitempty"`
	SubordinateAdditionalClaims []SubordinateAdditionalClaim    `gorm:"foreignKey:SubordinateID;constraint:OnDelete:CASCADE" json:"subordinate_additional_claims,omitempty"`
}

type BasicSubordinateInfo struct {
	ID                     uint                    `gorm:"primarykey" json:"id"`
	CreatedAt              int                     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt              int                     `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt              gorm.DeletedAt          `gorm:"index" json:"-"`
	EntityID               string                  `gorm:"uniqueIndex" json:"entity_id"`
	Description            string                  `gorm:"type:text" json:"description,omitempty"`
	SubordinateEntityTypes []SubordinateEntityType `gorm:"foreignKey:SubordinateID;constraint:OnDelete:CASCADE" json:"-"`
	Status                 Status                  `gorm:"index" json:"status"`
}

func (ExtendedSubordinateInfo) TableName() string { return "subordinates" }

// MarshalJSON customizes ExtendedSubordinateInfo JSON to expose entity types as []string
func (et SubordinateEntityType) MarshalJSON() ([]byte, error) {
	return json.Marshal(et.EntityType)
}

// UnmarshalJSON accepts entity_types as []string and populates join rows
func (s *SubordinateEntityType) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &s.EntityType)
}

// SubordinateEntityType is a join row mapping subordinates to entity type strings.
type SubordinateEntityType struct {
	SubordinateID uint   `gorm:"index;uniqueIndex:uidx_sub_ent" json:"-"`
	EntityType    string `gorm:"size:255;uniqueIndex:uidx_sub_ent" json:"entity_type"`
}

// Removed CritExtensions and subordinate_crit_extensions per db-fixes.

// PolicyOperator represents a policy operator in the database.
type PolicyOperator struct {
	ID             uint                      `gorm:"primarykey" json:"id"`
	CreatedAt      int                       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      int                       `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt            `gorm:"index" json:"-"`
	PolicyOperator oidfed.PolicyOperatorName `json:"policy_operator"`
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
			PolicyOperator: t,
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
			PolicyOperator: oidfed.PolicyOperatorName(t),
		}
	}
	return policyOperators
}

// ToStrings converts a PolicyOperators collection to a slice of strings.
// Each PolicyOperator is extracted into the resulting slice.
func (et PolicyOperators) ToStrings() []string {
	result := make([]string, len(et))
	for i, t := range et {
		result[i] = string(t.PolicyOperator)
	}
	return result
}

// ToPolicyOperatorNames converts a PolicyOperators collection to a slice of
// oidfed.PolicyOperatorName.
func (et PolicyOperators) ToPolicyOperatorNames() []oidfed.PolicyOperatorName {
	result := make([]oidfed.PolicyOperatorName, len(et))
	for i, t := range et {
		result[i] = t.PolicyOperator
	}
	return result
}
