package model

import (
	"gorm.io/gorm"
)

// Event type constants for subordinate events.
const (
	// EventTypeCreated is recorded when a subordinate is created.
	EventTypeCreated = "created"
	// EventTypeDeleted is recorded when a subordinate is deleted.
	EventTypeDeleted = "deleted"
	// EventTypeStatusUpdated is recorded when a subordinate's status changes.
	EventTypeStatusUpdated = "status_updated"
	// EventTypeUpdated is recorded when a subordinate's basic info is updated.
	EventTypeUpdated = "updated"
	// EventTypeJWKAdded is recorded when a JWK is added to a subordinate's JWKS.
	EventTypeJWKAdded = "jwk_added"
	// EventTypeJWKRemoved is recorded when a JWK is removed from a subordinate's JWKS.
	EventTypeJWKRemoved = "jwk_removed"
	// EventTypeJWKSReplaced is recorded when a subordinate's entire JWKS is replaced.
	EventTypeJWKSReplaced = "jwks_replaced"
	// EventTypeMetadataUpdated is recorded when subordinate metadata is updated.
	EventTypeMetadataUpdated = "metadata_updated"
	// EventTypeMetadataDeleted is recorded when subordinate metadata is deleted.
	EventTypeMetadataDeleted = "metadata_deleted"
	// EventTypePolicyUpdated is recorded when subordinate metadata policy is updated.
	EventTypePolicyUpdated = "policy_updated"
	// EventTypePolicyDeleted is recorded when subordinate metadata policy is deleted.
	EventTypePolicyDeleted = "policy_deleted"
	// EventTypeConstraintsUpdated is recorded when subordinate constraints are updated.
	EventTypeConstraintsUpdated = "constraints_updated"
	// EventTypeConstraintsDeleted is recorded when subordinate constraints are deleted.
	EventTypeConstraintsDeleted = "constraints_deleted"
	// EventTypeClaimsUpdated is recorded when subordinate additional claims are updated.
	EventTypeClaimsUpdated = "claims_updated"
	// EventTypeClaimDeleted is recorded when a subordinate additional claim is deleted.
	EventTypeClaimDeleted = "claim_deleted"
	// EventTypeLifetimeUpdated is recorded when subordinate lifetime is updated.
	EventTypeLifetimeUpdated = "lifetime_updated"
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
