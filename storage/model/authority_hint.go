package model

import (
	"gorm.io/gorm"
)

// AuthorityHint records the entity IDs of superiors published in the entity configuration.
type AuthorityHint struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   int            `json:"created_at"`
	UpdatedAt   int            `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	EntityID    string         `gorm:"uniqueIndex" json:"entity_id"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
}

type AddAuthorityHint struct {
	EntityID    string `json:"entity_id"`
	Description string `json:"description"`
}

// AuthorityHintsStore is the abstraction used by handlers.
type AuthorityHintsStore interface {
	List() ([]AuthorityHint, error)
	Create(hint AddAuthorityHint) (*AuthorityHint, error)
	Get(ident string) (*AuthorityHint, error)
	Update(ident string, update AddAuthorityHint) (*AuthorityHint, error)
	Delete(ident string) error
}
