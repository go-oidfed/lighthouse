package model

import (
	"gorm.io/gorm"
)

// SubordinateEvent stores an event related to a subordinate.
type SubordinateEvent struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     int            `json:"created_at"`
	UpdatedAt     int            `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	SubordinateID uint           `gorm:"index" json:"subordinate_id"`
	Timestamp     int64          `gorm:"index" json:"timestamp"`
	Type          string         `gorm:"index" json:"type"`
	Status        *string        `json:"status,omitempty"`
	Message       *string        `json:"message,omitempty"`
	Actor         *string        `json:"actor,omitempty"`
}
