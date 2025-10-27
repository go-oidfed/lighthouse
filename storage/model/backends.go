package model

// Backends groups all storage interfaces used by the application.
// It provides a single struct that can be passed around instead of
// multiple return values for each storage backend.
type Backends struct {
	Subordinates     SubordinateStorageBackend
	TrustMarks       TrustMarkedEntitiesStorageBackend
	AuthorityHints   AuthorityHintsStore
	AdditionalClaims AdditionalClaimsStore
	KV               KeyValueStore
}
