package model

import (
	"time"
)

// JTIUsed represents a used JWT ID for replay attack prevention
type JTIUsed struct {
	JTI       string    `gorm:"primaryKey;size:255"`
	ExpiresAt time.Time `gorm:"index:idx_expires_at"`
	CreatedAt time.Time
}

// TableName returns the table name for JTIUsed
func (JTIUsed) TableName() string {
	return "jtis_used"
}
