package main

import (
	"encoding/json"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/go-oidfed/lighthouse/storage/model"
)

type legacySubordinateInfo struct {
	EntityID           string                          `json:"entity_id" gorm:"uniqueIndex;size:255"`
	JWKS               jwx.JWKS                        `json:"jwks" gorm:"serializer:json"`
	EntityTypes        []string                        `json:"entity_types" gorm:"-"`
	Metadata           *oidfed.Metadata                `json:"metadata,omitempty" gorm:"serializer:json"`
	MetadataPolicy     *oidfed.MetadataPolicies        `json:"metadata_policy,omitempty" gorm:"serializer:json"`
	Constraints        *oidfed.ConstraintSpecification `json:"constraints,omitempty" gorm:"serializer:json"`
	MetadataPolicyCrit []oidfed.PolicyOperatorName     `json:"metadata_policy_crit,omitempty"`
	Status             model.Status                    `json:"status"`
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (info *legacySubordinateInfo) UnmarshalJSON(src []byte) error {
	type subordinateInfo legacySubordinateInfo
	ii := subordinateInfo(*info)
	if err := json.Unmarshal(src, &ii); err != nil {
		return err
	}
	*info = legacySubordinateInfo(ii)
	return nil
}

// UnmarshalMsgpack implements the msgpack.Unmarshaler interface
func (info *legacySubordinateInfo) UnmarshalMsgpack(src []byte) error {
	type subordinateInfo legacySubordinateInfo
	ii := subordinateInfo(*info)
	if err := msgpack.Unmarshal(src, &ii); err != nil {
		return err
	}
	*info = legacySubordinateInfo(ii)
	return nil
}

type loadLegacySubordinateInfos func() ([]legacySubordinateInfo, error)
