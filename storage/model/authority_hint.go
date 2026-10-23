package model

import (
	"errors"

	"gorm.io/gorm"
)

// JWKSSyncMode selects how (if at all) Lighthouse's rotated federation keys are
// synced to a superior (authority hint) after a key rotation.
type JWKSSyncMode string

const (
	// JWKSSyncNone disables syncing. This is the default (jwks_sync omitted).
	JWKSSyncNone JWKSSyncMode = "none"
	// JWKSSyncPush pushes a signed JWK Set (application/jwk-set+jwt) to the
	// superior's federation_jwks_update_endpoint. The endpoint URL and
	// acceptable signing algorithms are resolved dynamically from the
	// superior's Entity Configuration on each rotation. Corresponds to
	// oidfed.JWKSUpdateHook in the go-oidfed lib.
	JWKSSyncPush JWKSSyncMode = "push"
	// JWKSSyncTrigger POSTs this entity's entity_id to the superior's
	// federation_jwks_update_trigger_endpoint (authenticated with
	// private_key_jwt), telling it to re-fetch this entity's JWKS from its
	// Entity Configuration. Corresponds to oidfed.TriggerUpdateHook in the
	// go-oidfed lib.
	JWKSSyncTrigger JWKSSyncMode = "trigger"
)

// JWKSSyncConf configures optional syncing of Lighthouse's rotated federation
// keys to a superior authority hint. It only takes effect when the superior's
// Entity Configuration publishes the matching federation_jwks_update endpoint.
// Durations are integer seconds, matching the convention used across the admin
// API (e.g. key_poll_interval, jwks_poll_interval).
type JWKSSyncConf struct {
	// Mode selects the sync mechanism. Default: none.
	Mode JWKSSyncMode `json:"mode"`
	// JWTLifetime is the lifetime (exp - iat), in seconds, of the signed JWKS
	// JWT sent in push mode. Default: 600 (10 minutes). Ignored in other modes.
	JWTLifetime int64 `json:"jwt_lifetime,omitempty"`
	// Timeout is the HTTP client timeout for the sync request, in seconds.
	// Default: 20.
	Timeout int64 `json:"timeout,omitempty"`
	// Headers are additional HTTP headers set on the sync request.
	Headers map[string]string `json:"headers,omitempty"`
}

// Validate checks the sync config: the mode must be known.
func (c *JWKSSyncConf) Validate() error {
	switch c.Mode {
	case "", JWKSSyncNone, JWKSSyncPush, JWKSSyncTrigger:
		return nil
	default:
		return errors.New(
			"jwks_sync: unknown mode (supported: none, push, trigger)",
		)
	}
}

// AuthorityHint records the entity IDs of superiors published in the entity configuration.
type AuthorityHint struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   int            `json:"created_at"`
	UpdatedAt   int            `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	EntityID    string         `gorm:"size:255;uniqueIndex" json:"entity_id"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	// JWKSSync optionally configures syncing of rotated federation keys to this
	// superior. Stored as a JSON column; nil means no syncing. This is an
	// outbound directive only and is never published in the entity
	// configuration.
	JWKSSync *JWKSSyncConf `gorm:"serializer:json" json:"jwks_sync,omitempty"`
}

type AddAuthorityHint struct {
	EntityID    string        `json:"entity_id"`
	Description string        `json:"description"`
	JWKSSync    *JWKSSyncConf `json:"jwks_sync,omitempty"`
}

// AuthorityHintsStore is the abstraction used by handlers.
type AuthorityHintsStore interface {
	List() ([]AuthorityHint, error)
	Create(hint AddAuthorityHint) (*AuthorityHint, error)
	Get(ident string) (*AuthorityHint, error)
	Update(ident string, update AddAuthorityHint) (*AuthorityHint, error)
	Delete(ident string) error
}
