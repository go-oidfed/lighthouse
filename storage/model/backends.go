package model

import (
	"github.com/go-oidfed/lib/jwx/keymanagement/public"
)

// Backends groups all storage interfaces used by the application.
// It provides a single struct that can be passed around instead of
// multiple return values for each storage backend.
type Backends struct {
	Subordinates     SubordinateStorageBackend
	TrustMarks       TrustMarkedEntitiesStorageBackend
	AuthorityHints   AuthorityHintsStore
	TrustMarkTypes   TrustMarkTypesStore
	TrustMarkOwners  TrustMarkOwnersStore
	TrustMarkIssuers TrustMarkIssuersStore
	AdditionalClaims AdditionalClaimsStore
	KV               KeyValueStore
	Users            UsersStore
	PKStorages       func(string) public.PublicKeyStorage
}
