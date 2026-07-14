package lighthouse

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/cache"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"

	"github.com/go-oidfed/lighthouse/internal"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// registeredEndpoint holds a handler and its auth middleware (if any) for a
// federation endpoint managed by the endpoint registry.
type registeredEndpoint struct {
	Type    model.FederationEndpointType
	Path    string
	Method  string // fiber.MethodGet or fiber.MethodPost
	Handler fiber.Handler
	Auth    fiber.Handler // nil if no auth
}

// legacyEntry is a registeredEndpoint kept alive at an old path for one entity
// configuration lifetime after the endpoint was renamed, deleted, or disabled.
type legacyEntry struct {
	ep        *registeredEndpoint
	expiresAt time.Time
}

// EndpointRegistry holds in-memory handler registrations for all federation
// endpoints, keyed by path. It is rebuilt from the database on changes.
//
// The legacy map keeps old endpoint paths served for one entity configuration
// lifetime after a path change or deletion, so clients holding a cached entity
// configuration that still references the old path can continue to use it.
type EndpointRegistry struct {
	mu     sync.RWMutex
	byPath map[string]*registeredEndpoint
	byType map[model.FederationEndpointType]*registeredEndpoint
	legacy map[string]*legacyEntry
}

// NewEndpointRegistry creates a new empty EndpointRegistry.
func NewEndpointRegistry() *EndpointRegistry {
	return &EndpointRegistry{
		byPath: make(map[string]*registeredEndpoint),
		byType: make(map[model.FederationEndpointType]*registeredEndpoint),
		legacy: make(map[string]*legacyEntry),
	}
}

// register adds or replaces an endpoint in the registry.
func (r *EndpointRegistry) register(ep *registeredEndpoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Remove old path entry if this type is being re-registered with a different path.
	if existing, ok := r.byType[ep.Type]; ok && existing.Path != ep.Path {
		delete(r.byPath, existing.Path)
	}
	r.byPath[ep.Path] = ep
	r.byType[ep.Type] = ep
}

// unregister removes an endpoint by type.
func (r *EndpointRegistry) unregister(t model.FederationEndpointType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.byType[t]; ok {
		delete(r.byPath, existing.Path)
		delete(r.byType, t)
	}
}

// lookup finds an endpoint by path. Returns nil if not found.
func (r *EndpointRegistry) lookup(path string) *registeredEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byPath[path]
}

// lookupByType finds an endpoint by type. Returns nil if not found.
func (r *EndpointRegistry) lookupByType(t model.FederationEndpointType) *registeredEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byType[t]
}

// paths returns all registered paths.
func (r *EndpointRegistry) paths() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	paths := make([]string, 0, len(r.byPath))
	for p := range r.byPath {
		paths = append(paths, p)
	}
	return paths
}

// count returns the number of registered endpoints.
func (r *EndpointRegistry) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byType)
}

// registerLegacy adds a legacy entry: an old endpoint path kept alive until
// expiresAt so clients with a cached entity configuration can still reach it.
func (r *EndpointRegistry) registerLegacy(ep *registeredEndpoint, expiresAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.legacy[ep.Path] = &legacyEntry{ep: ep, expiresAt: expiresAt}
}

// lookupLegacy returns the registered endpoint for a legacy path, or nil if
// no legacy entry exists or it has expired. Expired entries are pruned lazily.
func (r *EndpointRegistry) lookupLegacy(path string) *registeredEndpoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.legacy[path]
	if !ok {
		return nil
	}
	if time.Now().After(entry.expiresAt) {
		delete(r.legacy, path)
		return nil
	}
	return entry.ep
}

// activeEntries returns a snapshot of all currently active registered endpoints.
func (r *EndpointRegistry) activeEntries() []*registeredEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := make([]*registeredEndpoint, 0, len(r.byType))
	for _, ep := range r.byType {
		entries = append(entries, ep)
	}
	return entries
}

// legacyEntries returns a snapshot of all legacy entries (including expired
// ones, so the caller can decide what to prune).
func (r *EndpointRegistry) legacyEntries() []*legacyEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := make([]*legacyEntry, 0, len(r.legacy))
	for _, le := range r.legacy {
		entries = append(entries, le)
	}
	return entries
}

// HasEndpoint reports whether an endpoint of the given type is registered.
func (fed *LightHouse) HasEndpoint(t model.FederationEndpointType) bool {
	if fed.endpointRegistry == nil {
		return false
	}
	return fed.endpointRegistry.lookupByType(t) != nil
}

// EndpointRegistry returns the endpoint registry (for testing/inspection).
func (fed *LightHouse) EndpointRegistry() *EndpointRegistry {
	return fed.endpointRegistry
}

// ReloadEndpointsFromDB clears the endpoint registry and reloads all
// federation endpoints from the database. It also stops/restarts background
// services and invalidates the entity configuration cache so published
// metadata reflects the new paths.
//
// Old endpoint paths that were renamed, deleted, or disabled are kept alive
// in the legacy map for one entity configuration lifetime so clients holding
// a cached entity configuration can still reach them.
func (fed *LightHouse) ReloadEndpointsFromDB() error {
	// Stop existing background services.
	fed.stopBackgroundServices()

	// Snapshot the old registry's active and legacy entries before
	// replacing it, so we can compute which old paths need a grace period.
	var oldActive []*registeredEndpoint
	var oldLegacy []*legacyEntry
	oldRegistry := fed.endpointRegistry
	if oldRegistry != nil {
		oldActive = oldRegistry.activeEntries()
		oldLegacy = oldRegistry.legacyEntries()
	}

	// Build a new registry in a temporary variable, then swap atomically.
	// This avoids serving from a partially-populated registry.
	fed.endpointRegistry = NewEndpointRegistry()
	if err := fed.LoadEndpointsFromDB(); err != nil {
		// On error, restore the old registry.
		fed.endpointRegistry = oldRegistry
		return err
	}

	// Compute legacy entries: keep old paths alive for one entity
	// configuration lifetime when they were renamed, deleted, or disabled.
	newRegistry := fed.endpointRegistry
	gracePeriod := fed.entityConfigurationLifetime()
	now := time.Now()
	freshLegacyPaths := make(map[string]bool)

	// Add a legacy entry for each old active endpoint whose path changed
	// or disappeared from the new registry.
	for _, oldEP := range oldActive {
		newEP := newRegistry.lookupByType(oldEP.Type)
		if newEP != nil && newEP.Path == oldEP.Path {
			continue // same path, still active — no legacy needed
		}
		// Old path is gone or changed — keep it alive.
		newRegistry.registerLegacy(oldEP, now.Add(gracePeriod))
		freshLegacyPaths[oldEP.Path] = true
	}

	// Carry over still-valid legacy entries from the old registry,
	// pruning expired ones. Skip paths that are now active or were already
	// re-added as fresh legacy entries above.
	for _, le := range oldLegacy {
		if now.After(le.expiresAt) {
			continue // expired, drop
		}
		if freshLegacyPaths[le.ep.Path] {
			continue // already re-added with fresh expiry
		}
		if newRegistry.lookup(le.ep.Path) != nil {
			continue // path is now active, don't shadow it
		}
		newRegistry.registerLegacy(le.ep, le.expiresAt)
	}

	// Invalidate entity configuration cache so published metadata updates.
	_ = cache.Delete(internal.CacheKeyEntityConfiguration)
	return nil
}

// entityConfigurationLifetime returns the configured entity configuration
// lifetime, falling back to the default if the KV store is unavailable.
func (fed *LightHouse) entityConfigurationLifetime() time.Duration {
	lifetime, err := storage.GetEntityConfigurationLifetime(fed.storages.KV)
	if err != nil || lifetime <= 0 {
		return storage.DefaultEntityConfigurationLifetime
	}
	return lifetime
}

// stopBackgroundServices stops all running background service goroutines.
func (fed *LightHouse) stopBackgroundServices() {
	for _, stop := range fed.backgroundStops {
		stop()
	}
	fed.backgroundStops = nil
}

// startBackgroundServicesForEndpoints starts background services (periodic
// entity collector) for endpoints that need them. Called after endpoints are
// loaded from the DB. Parent process only.
func (fed *LightHouse) startBackgroundServicesForEndpoints() {
	if fed.storages.FederationEndpoints == nil {
		return
	}
	endpoints, err := fed.storages.FederationEndpoints.List()
	if err != nil {
		log.WithError(err).Error("failed to list endpoints for background services")
		return
	}
	for _, ep := range endpoints {
		if ep.Path == nil || *ep.Path == "" {
			continue
		}
		if ep.Type == model.EndpointTypeEntityCollection {
			var cfg collectionDBConfig
			if ep.Config != "" {
				if err := json.Unmarshal([]byte(ep.Config), &cfg); err != nil {
					log.WithError(err).WithField("type", ep.Type).
						Error("failed to parse entity_collection config for background service")
					continue
				}
			}
			if cfg.IntervalSeconds > 0 {
				pec := &oidfed.PeriodicEntityCollector{
					TrustAnchors: cfg.AllowedTrustAnchors,
					Interval:     time.Duration(cfg.IntervalSeconds) * time.Second,
					Concurrency:  cfg.ConcurrencyLimit,
				}
				if cfg.PaginationLimit > 0 {
					pec.SortEntitiesComparisonFunc = func(a, b *oidfed.CollectedEntity) int {
						return strings.Compare(a.EntityID, b.EntityID)
					}
					pec.PagingLimit = cfg.PaginationLimit
				}
				pec.Start()
				fed.backgroundStops = append(fed.backgroundStops, pec.Stop)
				log.WithField("interval", pec.Interval).Info("Started periodic entity collector")
			}
		}
	}
}

// registerEndpoint is a helper called by Add*Endpoint methods to register
// their handler in the registry instead of directly on the fiber server.
func (fed *LightHouse) registerEndpoint(
	t model.FederationEndpointType, path string, method string,
	handler fiber.Handler, auth fiber.Handler,
) {
	if fed.endpointRegistry == nil {
		fed.endpointRegistry = NewEndpointRegistry()
	}
	fed.endpointRegistry.register(&registeredEndpoint{
		Type:    t,
		Path:    path,
		Method:  method,
		Handler: handler,
		Auth:    auth,
	})
}

// unregisterEndpoint removes an endpoint from the registry by type.
func (fed *LightHouse) unregisterEndpoint(t model.FederationEndpointType) {
	if fed.endpointRegistry == nil {
		return
	}
	fed.endpointRegistry.unregister(t)
}

// dispatch is the catch-all handler that serves all federation endpoints from
// the registry. It is registered once on the fiber server.
func (fed *LightHouse) dispatch(ctx *fiber.Ctx) error {
	path := ctx.Path()

	// Skip well-known and admin paths — they have their own routes.
	if path == "/.well-known/openid-federation" ||
		len(path) >= len("/api/v1/admin") && path[:len("/api/v1/admin")] == "/api/v1/admin" {
		return ctx.Next()
	}

	ep := fed.endpointRegistry.lookup(path)
	if ep == nil {
		// Fall back to legacy paths: old endpoint locations kept alive
		// for one entity configuration lifetime after a path change or
		// deletion.
		ep = fed.endpointRegistry.lookupLegacy(path)
	}
	if ep == nil {
		return ctx.Status(fiber.StatusNotFound).JSON(
			map[string]string{
				"error":             "not_found",
				"error_description": fmt.Sprintf("no endpoint registered for path %s", path),
			},
		)
	}

	if ep.Method != ctx.Method() {
		return ctx.Status(fiber.StatusMethodNotAllowed).JSON(
			map[string]string{
				"error":             "method_not_allowed",
				"error_description": fmt.Sprintf("endpoint %s requires %s", path, ep.Method),
			},
		)
	}

	if ep.Auth != nil {
		if err := ep.Auth(ctx); err != nil {
			return err
		}
	}
	return ep.Handler(ctx)
}

// LoadEndpointsFromDB loads all federation endpoints from the database and
// registers them via the Add*Endpoint methods. This replaces the config-file
// based registerEndpoints when endpoints are DB-managed.
func (fed *LightHouse) LoadEndpointsFromDB() error {
	if fed.storages.FederationEndpoints == nil {
		return nil
	}
	endpoints, err := fed.storages.FederationEndpoints.List()
	if err != nil {
		return fmt.Errorf("failed to list federation endpoints: %w", err)
	}

	loaded := 0
	for _, ep := range endpoints {
		if ep.Path == nil || *ep.Path == "" {
			continue
		}
		if err := fed.loadEndpointFromDB(&ep); err != nil {
			log.WithError(err).WithField("type", ep.Type).
				Error("failed to load endpoint from DB")
			continue
		}
		loaded++
	}

	// Start background services for the loaded endpoints.
	fed.startBackgroundServicesForEndpoints()

	log.WithField("count", loaded).Info("Loaded federation endpoints from DB")
	return nil
}

// loadEndpointFromDB loads a single endpoint from a DB row by dispatching to
// the appropriate factory based on the endpoint type.
func (fed *LightHouse) loadEndpointFromDB(ep *model.FederationEndpoint) error {
	endpointConf := dbEndpointToConf(ep)

	switch ep.Type {
	case model.EndpointTypeFetch:
		return fed.AddFetchEndpoint(endpointConf, fed.storages.Subordinates)

	case model.EndpointTypeList:
		return fed.AddSubordinateListingEndpoint(endpointConf, fed.storages.Subordinates, fed.storages.TrustMarks)

	case model.EndpointTypeResolve:
		var cfg resolveDBConfig
		if ep.Config != "" {
			if err := json.Unmarshal([]byte(ep.Config), &cfg); err != nil {
				return fmt.Errorf("failed to parse resolve config: %w", err)
			}
		}
		allowedTAs := cfg.AllowedTrustAnchors
		if cfg.UseEntityCollectionAllowedTrustAnchors {
			// Dynamically look up the entity collection endpoint's allowed TAs.
			collectionTAs, err := fed.resolveCollectionAllowedTAs()
			if err != nil {
				log.WithError(err).Warn("failed to resolve collection allowed TAs for resolve endpoint")
			} else if len(collectionTAs) > 0 {
				allowedTAs = collectionTAs
			}
		}
		// Set resolver cache globals from DB config.
		if cfg.GracePeriodSeconds > 0 {
			oidfed.ResolverCacheGracePeriod = time.Duration(cfg.GracePeriodSeconds) * time.Second
		}
		if cfg.TimeElapsedGraceFactor > 0 {
			oidfed.ResolverCacheLifetimeElapsedGraceFactor = cfg.TimeElapsedGraceFactor
		}
		var proactiveResolver *oidfed.ProactiveResolver
		if cfg.ProactiveResolver != nil && cfg.ProactiveResolver.Enabled {
			proactiveResolver = &oidfed.ProactiveResolver{
				EntityID: fed.FederationEntity.EntityID(),
				Store: oidfed.ResolveStore{
					BaseDir:   cfg.ProactiveResolver.ResponseStorageDir,
					StoreJWT:  cfg.ProactiveResolver.ResponseStorageStoreJWT,
					StoreJSON: cfg.ProactiveResolver.ResponseStorageStoreJSON,
				},
				Signer:      fed.ResolveResponseSigner(),
				RefreshLead: time.Duration(cfg.GracePeriodSeconds) * time.Second,
				Concurrency: cfg.ProactiveResolver.ConcurrencyLimit,
				QueueSize:   cfg.ProactiveResolver.QueueSize,
			}
			proactiveResolver.Start()
			fed.backgroundStops = append(fed.backgroundStops, proactiveResolver.Stop)
		}
		return fed.AddResolveEndpoint(endpointConf, allowedTAs, proactiveResolver)

	case model.EndpointTypeTrustMarkStatus:
		return fed.AddTrustMarkStatusEndpoint(endpointConf, TrustMarkStatusConfig{
			InstanceStore: fed.storages.TrustMarkInstances,
		})

	case model.EndpointTypeTrustMarkListing:
		return fed.AddTrustMarkedEntitiesListingEndpoint(endpointConf, fed.storages.TrustMarkInstances)

	case model.EndpointTypeTrustMark:
		eligibilityCache := NewEligibilityCache()
		stopEligibilityCacheCleanup := eligibilityCache.StartCleanupRoutine(5 * time.Minute)
		_ = stopEligibilityCacheCleanup // TODO: manage lifecycle
		issuedTrustMarkCache := NewIssuedTrustMarkCache()
		stopIssuedCacheCleanup := issuedTrustMarkCache.StartCleanupRoutine(5 * time.Minute)
		_ = stopIssuedCacheCleanup // TODO: manage lifecycle
		return fed.AddTrustMarkEndpointWithConfig(endpointConf, TrustMarkEndpointConfig{
			Store:                fed.storages.TrustMarks,
			SpecStore:            fed.storages.TrustMarkSpecs,
			InstanceStore:        fed.storages.TrustMarkInstances,
			Cache:                eligibilityCache,
			IssuedTrustMarkCache: issuedTrustMarkCache,
		})

	case model.EndpointTypeTrustMarkRequest:
		return fed.AddTrustMarkRequestEndpoint(endpointConf, fed.storages.TrustMarks)

	case model.EndpointTypeHistoricalKeys:
		return fed.AddHistoricalKeysEndpoint(endpointConf)

	case model.EndpointTypeEnroll:
		var cfg enrollDBConfig
		if ep.Config != "" {
			if err := json.Unmarshal([]byte(ep.Config), &cfg); err != nil {
				return fmt.Errorf("failed to parse enroll config: %w", err)
			}
		}
		var checker EntityChecker
		if cfg.CheckerType != "" {
			c, err := EntityCheckerFromJSONConfig(cfg.CheckerType, cfg.CheckerConfig)
			if err != nil {
				return fmt.Errorf("failed to create entity checker: %w", err)
			}
			checker = c
		}
		return fed.AddEnrollEndpoint(endpointConf, fed.storages.Subordinates, checker)

	case model.EndpointTypeEnrollRequest:
		return fed.AddEnrollRequestEndpoint(endpointConf, fed.storages.Subordinates)

	case model.EndpointTypeEntityCollection:
		var cfg collectionDBConfig
		if ep.Config != "" {
			if err := json.Unmarshal([]byte(ep.Config), &cfg); err != nil {
				return fmt.Errorf("failed to parse entity_collection config: %w", err)
			}
		}
		var collector oidfed.EntityCollector = &oidfed.SimpleEntityCollector{}
		if cfg.IntervalSeconds > 0 {
			pec := &oidfed.PeriodicEntityCollector{
				TrustAnchors: cfg.AllowedTrustAnchors,
				Interval:     time.Duration(cfg.IntervalSeconds) * time.Second,
				Concurrency:  cfg.ConcurrencyLimit,
			}
			if cfg.PaginationLimit > 0 {
				pec.SortEntitiesComparisonFunc = func(a, b *oidfed.CollectedEntity) int {
					return strings.Compare(a.EntityID, b.EntityID)
				}
				pec.PagingLimit = cfg.PaginationLimit
			}
			collector = pec
		}
		return fed.AddEntityCollectionEndpoint(
			endpointConf, collector, cfg.AllowedTrustAnchors, cfg.PaginationLimit > 0,
		)

	default:
		return fmt.Errorf("unknown endpoint type: %s", ep.Type)
	}
}

// dbEndpointToConf converts a DB FederationEndpoint to an EndpointConf,
// resolving auth trust anchor entity IDs from the join table.
func dbEndpointToConf(ep *model.FederationEndpoint) EndpointConf {
	conf := EndpointConf{
		AuthEnabled: ep.AuthEnabled,
	}
	if ep.Path != nil {
		conf.Path = *ep.Path
	}
	if ep.URL != nil {
		conf.URL = *ep.URL
	}
	for _, ta := range ep.AuthTrustAnchors {
		conf.AuthTrustAnchors = append(conf.AuthTrustAnchors, ta.EntityID)
	}
	return conf
}

// resolveCollectionAllowedTAs looks up the entity collection endpoint's
// allowed trust anchors from the database. This is used when the resolve
// endpoint has use_entity_collection_allowed_trust_anchors=true, so changes
// to the entity collection's TAs via the admin API automatically propagate
// to the resolve endpoint on the next reload.
func (fed *LightHouse) resolveCollectionAllowedTAs() ([]string, error) {
	if fed.storages.FederationEndpoints == nil {
		return nil, nil
	}
	collectionEP, err := fed.storages.FederationEndpoints.GetByType(model.EndpointTypeEntityCollection)
	if err != nil {
		return nil, fmt.Errorf("entity collection endpoint not found: %w", err)
	}
	if collectionEP.Config == "" {
		return nil, nil
	}
	var cfg collectionDBConfig
	if err := json.Unmarshal([]byte(collectionEP.Config), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse entity_collection config: %w", err)
	}
	return cfg.AllowedTrustAnchors, nil
}

// DB config structs for type-specific endpoint configuration stored as JSON
// in the FederationEndpoint.Config column.

type resolveDBConfig struct {
	AllowedTrustAnchors                    []string                   `json:"allowed_trust_anchors,omitempty"`
	UseEntityCollectionAllowedTrustAnchors bool                       `json:"use_entity_collection_allowed_trust_anchors,omitempty"`
	GracePeriodSeconds                     int64                      `json:"grace_period_seconds,omitempty"`
	TimeElapsedGraceFactor                 float64                    `json:"time_elapsed_grace_factor,omitempty"`
	ProactiveResolver                      *proactiveResolverDBConfig `json:"proactive_resolver,omitempty"`
}

type proactiveResolverDBConfig struct {
	Enabled                  bool   `json:"enabled"`
	ConcurrencyLimit         int    `json:"concurrency_limit,omitempty"`
	QueueSize                int    `json:"queue_size,omitempty"`
	ResponseStorageDir       string `json:"response_storage_dir,omitempty"`
	ResponseStorageStoreJSON bool   `json:"response_storage_store_json,omitempty"`
	ResponseStorageStoreJWT  bool   `json:"response_storage_store_jwt,omitempty"`
}

type enrollDBConfig struct {
	CheckerType   string `json:"checker_type,omitempty"`
	CheckerConfig any    `json:"checker_config,omitempty"`
}

type collectionDBConfig struct {
	AllowedTrustAnchors []string `json:"allowed_trust_anchors,omitempty"`
	IntervalSeconds     int64    `json:"interval_seconds,omitempty"`
	ConcurrencyLimit    int      `json:"concurrency_limit,omitempty"`
	PaginationLimit     int      `json:"pagination_limit,omitempty"`
}
