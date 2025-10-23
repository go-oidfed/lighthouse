package model

import (
	"gorm.io/gorm"
)

// EntityType represents an entity type in the database.
type EntityType struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  int            `json:"created_at"`
	UpdatedAt  int            `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	EntityType string         `gorm:"uniqueIndex" json:"entity_type"`
}

// EntityTypes is a collection of EntityType objects.
// This type provides methods for working with multiple entity types together.
type EntityTypes []EntityType

// NewEntityTypes creates a new EntityTypes collection from a slice of strings.
// Each string is converted to an EntityType object.
func NewEntityTypes(types []string) EntityTypes {
	entityTypes := make(EntityTypes, len(types))
	for i, t := range types {
		entityTypes[i] = EntityType{
			EntityType: t,
		}
	}
	return entityTypes
}

// ToStrings converts an EntityTypes collection to a slice of strings.
// Each EntityType's EntityType field is extracted into the resulting slice.
func (et EntityTypes) ToStrings() []string {
	result := make([]string, len(et))
	for i, t := range et {
		result[i] = t.EntityType
	}
	return result
}
