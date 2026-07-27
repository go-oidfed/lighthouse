package model

import (
	"encoding/json"

	oidfed "github.com/go-oidfed/lib"
	"gorm.io/gorm"
)

// ExtendedSubordinateInfo holds information about a subordinate for storage
// Table name is set to `subordinates` to replace legacy `subordinate_infos`.
// Note: MetadataPolicyCrit is stored globally in the KV store, not per-subordinate.
type ExtendedSubordinateInfo struct {
	BasicSubordinateInfo
	JWKSID                      *uint                           `json:"-" gorm:"null"`
	JWKS                        JWKS                            `json:"jwks"`
	Metadata                    *oidfed.Metadata                `gorm:"serializer:json" json:"metadata,omitempty"`
	MetadataPolicy              *oidfed.MetadataPolicies        `gorm:"serializer:json" json:"metadata_policy,omitempty"`
	Constraints                 *oidfed.ConstraintSpecification `gorm:"serializer:json" json:"constraints,omitempty"`
	SubordinateAdditionalClaims []SubordinateAdditionalClaim    `gorm:"foreignKey:SubordinateID;constraint:OnDelete:CASCADE" json:"additional_claims,omitempty"`
}

type BasicSubordinateInfo struct {
	ID                     uint                    `gorm:"primarykey" json:"id"`
	CreatedAt              int                     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt              int                     `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt              gorm.DeletedAt          `gorm:"index" json:"-"`
	EntityID               string                  `gorm:"size:255;uniqueIndex" json:"entity_id"`
	Description            string                  `gorm:"type:text" json:"description,omitempty"`
	SubordinateEntityTypes []SubordinateEntityType `gorm:"foreignKey:SubordinateID;constraint:OnDelete:CASCADE" json:"registered_entity_types,omitempty"`
	Status                 Status                  `gorm:"index" json:"status"`
	// EnableJWKSUpdate controls whether Lighthouse periodically polls this
	// subordinate's Entity Configuration for JWKS changes (approach A) and
	// whether the subordinate is a valid target for the jwks_update_trigger
	// endpoint (approach B).
	EnableJWKSUpdate bool `gorm:"default:false" json:"enable_jwks_update,omitempty"`
	// JWKSPollInterval, when non-nil, is the per-subordinate polling interval
	// in seconds used by the subordinate JWKS refresher. When nil, the
	// refresher derives the interval from the subordinate's Entity
	// Configuration expiration time.
	JWKSPollInterval *int64 `gorm:"null" json:"jwks_poll_interval,omitempty"`
}

func (ExtendedSubordinateInfo) TableName() string { return "subordinates" }

// MarshalJSON customizes ExtendedSubordinateInfo JSON serialization.
// It transforms additional_claims from an array of objects to a simple {"claim": "value"} object.
func (e ExtendedSubordinateInfo) MarshalJSON() ([]byte, error) {
	type Alias ExtendedSubordinateInfo

	additionalClaimsMap := make(map[string]any, len(e.SubordinateAdditionalClaims))
	for _, claim := range e.SubordinateAdditionalClaims {
		additionalClaimsMap[claim.Claim] = claim.Value
	}

	return json.Marshal(&struct {
		AdditionalClaims map[string]any `json:"additional_claims,omitempty"`
		*Alias
	}{
		AdditionalClaims: additionalClaimsMap,
		Alias:            (*Alias)(&e),
	})
}

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

// AddSubordinate represents the payload for creating a new subordinate.
type AddSubordinate struct {
	EntityID              string   `json:"entity_id"`
	Status                Status   `json:"status"`
	Description           string   `json:"description,omitempty"`
	RegisteredEntityTypes []string `json:"registered_entity_types,omitempty"`
	JWKS                  *JWKS    `json:"jwks,omitempty"`
	EnableJWKSUpdate      bool     `json:"enable_jwks_update,omitempty"`
	JWKSPollInterval      *int64   `json:"jwks_poll_interval,omitempty"`
}

// UpdateSubordinate represents the payload for updating a subordinate.
type UpdateSubordinate struct {
	Description           *string       `json:"description,omitempty"`
	RegisteredEntityTypes []string      `json:"registered_entity_types,omitempty"`
	EnableJWKSUpdate      *bool         `json:"enable_jwks_update,omitempty"`
	JWKSPollInterval      NullableInt64 `json:"jwks_poll_interval,omitempty"`
}

// NullableInt64 is an int64 field that can distinguish between "key absent"
// and "key present as null" in JSON. This is needed for partial updates where
// `null` should clear the stored value while an omitted key should leave it
// unchanged.
//
//   - JSON key absent  → Defined=false, Value=nil
//   - JSON null         → Defined=true,  Value=nil  (clears the stored value)
//   - JSON integer      → Defined=true,  Value=&v   (sets the stored value)
type NullableInt64 struct {
	Defined bool
	Value   *int64
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *NullableInt64) UnmarshalJSON(b []byte) error {
	n.Defined = true
	if string(b) == "null" {
		n.Value = nil
		return nil
	}
	var v int64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	n.Value = &v
	return nil
}

// MarshalJSON implements json.Marshaler so the type serializes like a plain
// nullable int64 (null when Value is nil).
func (n NullableInt64) MarshalJSON() ([]byte, error) {
	if n.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.Value)
}
