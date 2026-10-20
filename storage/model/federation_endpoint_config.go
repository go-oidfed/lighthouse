package model

// JSON config structs for the type-specific configuration of federation
// endpoints, stored as a JSON blob in the FederationEndpoint.Config column
// and interpreted by the endpoint type's handler factory.

// ResolveDBConfigJSON is the JSON config struct for the resolve endpoint's
// type-specific config stored in the federation_endpoints table.
type ResolveDBConfigJSON struct {
	AllowedTrustAnchors                    []string                       `json:"allowed_trust_anchors,omitempty"`
	UseEntityCollectionAllowedTrustAnchors bool                           `json:"use_entity_collection_allowed_trust_anchors,omitempty"`
	GracePeriodSeconds                     int64                          `json:"grace_period_seconds,omitempty"`
	TimeElapsedGraceFactor                 float64                        `json:"time_elapsed_grace_factor,omitempty"`
	ProactiveResolver                      *ProactiveResolverDBConfigJSON `json:"proactive_resolver,omitempty"`
}

// ProactiveResolverDBConfigJSON is the JSON config struct for the proactive
// resolver sub-config of the resolve endpoint.
type ProactiveResolverDBConfigJSON struct {
	Enabled                  bool   `json:"enabled"`
	ConcurrencyLimit         int    `json:"concurrency_limit,omitempty"`
	QueueSize                int    `json:"queue_size,omitempty"`
	ResponseStorageDir       string `json:"response_storage_dir,omitempty"`
	ResponseStorageStoreJSON bool   `json:"response_storage_store_json,omitempty"`
	ResponseStorageStoreJWT  bool   `json:"response_storage_store_jwt,omitempty"`
}

// EnrollDBConfigJSON is the JSON config struct for the enroll endpoint's
// type-specific config.
type EnrollDBConfigJSON struct {
	CheckerType   string `json:"checker_type,omitempty"`
	CheckerConfig any    `json:"checker_config,omitempty"`
}

// CollectionDBConfigJSON is the JSON config struct for the entity collection
// endpoint's type-specific config.
type CollectionDBConfigJSON struct {
	AllowedTrustAnchors        []string `json:"allowed_trust_anchors,omitempty"`
	IntervalSeconds            int64    `json:"interval_seconds,omitempty"`
	ConcurrencyLimit           int      `json:"concurrency_limit,omitempty"`
	PaginationLimit            int      `json:"pagination_limit,omitempty"`
	DisableDelegatedCollection bool     `json:"disable_delegated_collection,omitempty"`
}
