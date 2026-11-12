package model

import (
	"gorm.io/gorm"
)

// Event type constants for subordinate events.
const (
	// EventTypeCreated is recorded when a subordinate is created, including
	// when an entity enrolls through the federation_enroll_endpoint.
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
	// EventTypeJWKSRefreshed is recorded when a subordinate's JWKS are updated
	// by the periodic refresher (approach A) after detecting a change in the
	// subordinate's Entity Configuration.
	EventTypeJWKSRefreshed = "jwks_refreshed"
	// EventTypeJWKSUpdateTriggered is recorded when the
	// federation_jwks_update_trigger_endpoint (approach B) is called and LH
	// re-fetches the subordinate's JWKS from its Entity Configuration.
	EventTypeJWKSUpdateTriggered = "jwks_update_triggered"
	// EventTypeJWKSUpdated is recorded when the federation_jwks_update_endpoint
	// (approach C) accepts a signed JWK Set from the subordinate and updates
	// the stored JWKS.
	EventTypeJWKSUpdated = "jwks_updated"
	// EventTypeJoinRequested is recorded when an entity asks to become a
	// subordinate through the federation_enroll_request_endpoint.
	EventTypeJoinRequested = "join_requested"
	// EventTypeTrustMarkGranted is recorded when a trust mark subject becomes
	// active through the admin API, so the entity is eligible for the mark.
	EventTypeTrustMarkGranted = "trust_mark_granted"
	// EventTypeTrustMarkRevoked is recorded when a trust mark subject leaves
	// active or is deleted through the admin API, or when the admin API
	// revokes the issued instances of a trust mark.
	EventTypeTrustMarkRevoked = "trust_mark_revoked"
	// EventTypeTrustMarkIssued is recorded when the admin API issues a trust
	// mark immediately ("issue": true).
	EventTypeTrustMarkIssued = "trust_mark_issued"
	// EventTypeTrustMarkStatusUpdated is recorded when a trust mark subject
	// changes between statuses other than active (e.g. pending to blocked).
	EventTypeTrustMarkStatusUpdated = "trust_mark_status_updated"
)

// SubordinateEvent stores an event related to a subordinate or another entity.
// EntityID identifies the entity the event is about: events are kept when a
// subordinate is removed and stay findable by entity ID if it joins again.
// SubordinateID is 0 when the entity is not a subordinate of this authority.
type SubordinateEvent struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     int            `json:"created_at"`
	UpdatedAt     int            `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	SubordinateID uint           `gorm:"index" json:"subordinate_id"`
	EntityID      string         `gorm:"size:255;index" json:"entity_id"`
	Timestamp     int64          `gorm:"index" json:"timestamp"`
	Type          string         `gorm:"index" json:"type"`
	Status        *string        `json:"status,omitempty"`
	Message       *string        `json:"message,omitempty"`
	Actor         *string        `json:"actor,omitempty"`
}
