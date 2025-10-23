package model

import (
	"gorm.io/gorm"
)

// AuthorityHint records the entity IDs of superiors published in the entity configuration.
type AuthorityHint struct {
	gorm.Model
	EntityID    string `gorm:"uniqueIndex"`
	Description string `gorm:"type:text"`
}
