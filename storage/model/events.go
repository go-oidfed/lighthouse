package model

import (
	"gorm.io/gorm"
)

// SubordinateEvent stores an event related to a subordinate.
type SubordinateEvent struct {
	gorm.Model
	SubordinateID uint   `gorm:"index"`
	Timestamp     int64  `gorm:"index"`
	Type          string `gorm:"index"`
	Status        *string
	Message       *string
	Actor         *string
}
