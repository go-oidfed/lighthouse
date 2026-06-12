package model

import (
	"time"

	"github.com/go-oidfed/lib/jwx/keymanagement/public"
	"gorm.io/gorm"
)

// JTIStorageBackend defines the interface for JTI replay prevention storage
type JTIStorageBackend interface {
	// Exists checks if a JTI has been used
	Exists(jti string) (bool, error)
	// Store marks a JTI as used with expiration
	Store(jti string, expiresAt time.Time) error
	// Cleanup removes expired JTIs (optional, for periodic cleanup)
	Cleanup() error
}

// TransactionFunc is a function that executes within a database transaction.
// All storage backends in the provided Backends operate within the same transaction.
// If the function returns an error, the transaction is rolled back.
type TransactionFunc func(txBackends *Backends) error

// Backends groups all storage interfaces used by the application.
// It provides a single struct that can be passed around instead of
// multiple return values for each storage backend.
type Backends struct {
	DB                  *gorm.DB
	Subordinates        SubordinateStorageBackend
	SubordinateEvents   SubordinateEventStore
	TrustMarks          TrustMarkedEntitiesStorageBackend
	TrustMarkSpecs      TrustMarkSpecStore
	TrustMarkInstances  IssuedTrustMarkInstanceStore
	AuthorityHints      AuthorityHintsStore
	TrustMarkTypes      TrustMarkTypesStore
	TrustMarkOwners     TrustMarkOwnersStore
	TrustMarkIssuers    TrustMarkIssuersStore
	AdditionalClaims    AdditionalClaimsStore
	PublishedTrustMarks PublishedTrustMarksStore
	KV                  KeyValueStore
	Users               UsersStore
	PKStorages          func(string) public.PublicKeyStorage
	Stats               StatsStorageBackend
	JTI                 JTIStorageBackend

	// Transaction wraps multiple storage operations in a single DB transaction.
	// All backends provided to the TransactionFunc operate within the same transaction.
	// If the function returns an error, the transaction is rolled back.
	// This field is nil when backends are already within a transaction (no nested transactions).
	Transaction func(fn TransactionFunc) error
}

// InTransaction executes fn within a transaction if supported, otherwise runs directly.
// This provides a safe way to use transactions when available while maintaining
// backward compatibility with backends that don't support transactions.
func (b *Backends) InTransaction(fn TransactionFunc) error {
	if b.Transaction != nil {
		return b.Transaction(fn)
	}
	// No transaction support, run directly (for backward compatibility)
	return fn(b)
}
