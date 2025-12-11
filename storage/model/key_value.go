package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	KeyValueScopeGlobal               = ""
	KeyValueScopeEntityConfiguration  = "entity_configuration"
	KeyValueScopeSubordinateStatement = "subordinate_statement"
	KeyValueScopeSigning              = "signing"

	KeyValueKeyLifetime       = "lifetime"
	KeyValueKeyMetadataPolicy = "metadata_policy"
	KeyValueKeyMetadata       = "metadata"
	KeyValueKeyCriticalClaims = "crit"
	KeyValueKeyAlg            = "alg"
	KeyValueKeyRSAKeyLen      = "rsa_key_len"
	KeyValueKeyKeyRotation    = "key_rotation"
)

// KeyValue stores arbitrary key-value data.
//
// Values are serialized efficiently using GORM's json serializer, which
// leverages the database JSON type when available (e.g., PostgreSQL, MySQL),
// and falls back to TEXT in others (e.g., SQLite). The `Scope` field enables
// namespacing to avoid key collisions across different features.
type KeyValue struct {
	CreatedAt int            `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt int            `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Scope allows grouping keys by namespace; empty string is global scope.
	Scope string `gorm:"primaryKey" json:"scope"`

	// Key is the identifier within a scope.
	Key string `gorm:"primaryKey" json:"key"`

	// Value is stored as native JSON/JSONB (where supported) using datatypes.JSON.
	Value datatypes.JSON `json:"value"`
}

// KeyValueStore defines common operations for key-value storage.
// Implementations may apply additional semantics (e.g., default scope) but
// should honor the uniqueness of (scope,key) and JSON-serialized values.
type KeyValueStore interface {
	// Get retrieves the value for a (scope, key). Returns (nil, nil) if not found.
	Get(scope, key string) (datatypes.JSON, error)

	// GetAs retrieves and unmarshals the value for (scope, key) into out.
	// out must be a pointer to the target type. Returns (false, nil) if not found.
	GetAs(scope, key string, out any) (bool, error)

	// Set stores/replaces the value for a (scope, key).
	Set(scope, key string, value datatypes.JSON) error

	// SetAny marshals v to JSON and stores it at (scope, key).
	SetAny(scope, key string, v any) error

	// Delete removes the entry for a (scope, key). No error if missing.
	Delete(scope, key string) error
}
