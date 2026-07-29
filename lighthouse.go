package lighthouse

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/cache"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lighthouse/api/adminapi"
	apistats "github.com/go-oidfed/lighthouse/api/stats"
	"github.com/go-oidfed/lighthouse/internal"
	"github.com/go-oidfed/lighthouse/internal/stats"
	"github.com/go-oidfed/lighthouse/internal/utils"
	"github.com/go-oidfed/lighthouse/internal/version"
	"github.com/go-oidfed/lighthouse/middleware"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

const MaximumEntityConfigurationCachePeriod = 8 * time.Hour

// MaximumSubordinateStatementCachePeriod is the upper bound on how long a
// signed subordinate statement JWT is cached at the fetch endpoint. The actual
// cache TTL is further capped by the statement's own expiration and by the
// earliest published key expiration (see subordinateStatementCacheTTL).
const MaximumSubordinateStatementCachePeriod = 8 * time.Hour

// parseRequest populates req from query parameters (GET) and, for POST requests,
// also from the request body (form or JSON).
func parseRequest(ctx *fiber.Ctx, req interface{}) error {
	if err := ctx.QueryParser(req); err != nil {
		return err
	}
	if ctx.Method() == fiber.MethodPost {
		if err := ctx.BodyParser(req); err != nil {
			return err
		}
	}
	return nil
}

// EndpointConf is a type for configuring an endpoint with an internal and external path.
//
// Environment variables use the parent endpoint's prefix, e.g.:
//   - LH_ENDPOINTS_FETCH_PATH: Internal path for the fetch endpoint
//   - LH_ENDPOINTS_FETCH_URL: External URL for the fetch endpoint
type EndpointConf struct {
	// Path is the internal path for the endpoint.
	// Env: LH_ENDPOINTS_<ENDPOINT>_PATH
	//
	// NOTE: We intentionally omit the envconfig tag here. Using envconfig:"PATH"
	// would cause the library to also check the bare "PATH" env var as a fallback,
	// which collides with the system PATH and corrupts endpoint URLs.
	// By omitting the tag, envconfig uses the field name "Path" directly,
	// resulting in the correct LH_ENDPOINTS_<ENDPOINT>_PATH without fallback issues.
	Path string `yaml:"path"`
	// URL is the external URL for the endpoint.
	// Env: LH_ENDPOINTS_<ENDPOINT>_URL
	URL string `yaml:"url"`
	// AuthEnabled requires authentication for this endpoint (default: false).
	// When global endpoints.auth.all_require_auth is true, this is forced on.
	// Env: LH_ENDPOINTS_<ENDPOINT>_AUTH_ENABLED
	AuthEnabled bool `yaml:"auth_enabled" envconfig:"AUTH_ENABLED"`
	// AuthTrustAnchors is the list of trust anchor entity IDs for endpoint authentication.
	// These are resolved live from the TrustAnchorRepo at request time.
	// If empty when auth is enabled, falls back to global endpoints.auth.trust_anchors.
	// Env: LH_ENDPOINTS_<ENDPOINT>_AUTH_TRUST_ANCHORS (comma-separated)
	AuthTrustAnchors []string `yaml:"auth_trust_anchors" envconfig:"AUTH_TRUST_ANCHORS"`
}

// IsSet returns a bool indicating if this endpoint was configured or not
func (c EndpointConf) IsSet() bool {
	return c.Path != "" || c.URL != ""
}

// ValidateURL validates that an external URL is set,
// and if not prefixes the internal path with the passed rootURL and sets it
// at the external url
func (c *EndpointConf) ValidateURL(rootURL string) string {
	if c.URL == "" {
		c.URL, _ = url.JoinPath(rootURL, c.Path)
	}
	return c.URL
}

// LightHouse is a type a that represents a federation entity that can have multiple purposes (TA/IA + TMI, etc.)
type LightHouse struct {
	oidfed.FederationEntity
	*oidfed.TrustMarkIssuer
	*jwx.GeneralJWTSigner
	server                   *fiber.App
	adminAPIServer           *fiber.App
	serverConf               ServerConf
	fedMetadata              oidfed.FederationEntityMetadata
	keyManagement            adminapi.KeyManagement
	LogoBanner               bool
	VersionBanner            bool
	storages                 model.Backends
	statsCollector           *stats.Collector
	statsAggregator          *stats.Aggregator
	statsAggregatorCancel    context.CancelFunc
	trustMarkConfigProvider  *storage.TrustMarkConfigProvider
	trustAnchorRepo          *TrustAnchorRepo
	taJWKSRefresher          *oidfed.TAJWKSRefresher
	subordinateJWKSRefresher *oidfed.SubordinateJWKSRefresher
	endpointRegistry         *EndpointRegistry
	backgroundStops          []func()
	jtiCleanupStop           func()
}

// FiberServerConfig is the fiber.Config that is used to init the http fiber.App
var FiberServerConfig = fiber.Config{
	ReadTimeout:    3 * time.Second,
	WriteTimeout:   20 * time.Second,
	IdleTimeout:    150 * time.Second,
	ReadBufferSize: 8192,
	// WriteBufferSize: 4096,
	ErrorHandler: handleError,
	Network:      "tcp",
}

// NewLightHouse creates a new LightHouse
func NewLightHouse(
	serverConf ServerConf,
	entityID string,
	signingConf SigningConf,
	storages model.Backends,
	admin AdminAPIOptions,
	statsConfig apistats.Config,
) (
	*LightHouse,
	error,
) {
	keyManagement, err := initKey(entityID, signingConf, storages)
	if err != nil {
		return nil, err
	}

	versatileSigner, err := createVersatileSigner(keyManagement)
	if err != nil {
		return nil, err
	}

	generalSigner := jwx.NewGeneralJWTSigner(versatileSigner, keyManagement.BasicKeys.GetAlgs())

	server, err := initFiberServer(serverConf)
	if err != nil {
		return nil, err
	}

	statsCollector, err := initStatsCollector(statsConfig, storages, server)
	if err != nil {
		return nil, err
	}

	statsAggregator := initStatsAggregator(statsConfig, storages)

	trustMarkConfigProvider := storage.NewTrustMarkConfigProvider(
		storages.PublishedTrustMarks,
		entityID,
		"",
		func() *jwx.TrustMarkSigner { return generalSigner.TrustMarkSigner() },
	)

	entity := &LightHouse{
		TrustMarkIssuer:         oidfed.NewTrustMarkIssuer(entityID, generalSigner.TrustMarkSigner(), nil),
		GeneralJWTSigner:        generalSigner,
		server:                  server,
		serverConf:              serverConf,
		LogoBanner:              true,
		VersionBanner:           true,
		keyManagement:           keyManagement,
		storages:                storages,
		statsCollector:          statsCollector,
		statsAggregator:         statsAggregator,
		trustMarkConfigProvider: trustMarkConfigProvider,
	}

	entity.FederationEntity = buildDynamicFederationEntity(entity, entityID, storages)

	entity.endpointRegistry = NewEndpointRegistry()

	registerEntityConfigurationEndpoint(server, entity)

	// Register the catch-all dispatcher for federation endpoints.
	// Specific routes (/.well-known/openid-federation, /api/v1/admin/*) take
	// precedence in Fiber's radix tree; all other paths are dispatched via
	// the endpoint registry.
	server.All("/*", entity.dispatch)

	adminAPIServer, err := initAdminAPI(
		admin, serverConf, server, entity, entityID, storages,
		entity.FederationEntity, keyManagement, trustMarkConfigProvider,
	)
	if err != nil {
		return nil, err
	}
	entity.adminAPIServer = adminAPIServer
	entity.serverConf.AdminAPIPort = admin.Port

	return entity, nil
}

func createVersatileSigner(keyManagement adminapi.KeyManagement) (jwx.VersatileSigner, error) {
	return kms.KMSToVersatileSignerWithJWKSFunc(
		keyManagement.BasicKeys,
		func() (jwx.JWKS, error) {
			kmsHistory, err := keyManagement.KMSManagedPKs.GetValid()
			if err != nil {
				return jwx.JWKS{}, err
			}
			apiHistory, err := keyManagement.APIManagedPKs.GetValid()
			if err != nil {
				return jwx.JWKS{}, err
			}
			allEntries := append(kmsHistory, apiHistory...)
			set := jwx.NewJWKS()
			for _, k := range allEntries {
				kk, err := k.JWK()
				if err != nil {
					return jwx.JWKS{}, err
				}
				_ = set.AddKey(kk)
			}
			return set, nil
		},
	), nil
}

func initFiberServer(serverConf ServerConf) (*fiber.App, error) {
	if tps := serverConf.TrustedProxies; len(tps) > 0 {
		FiberServerConfig.TrustedProxies = serverConf.TrustedProxies
		FiberServerConfig.EnableTrustedProxyCheck = true
		FiberServerConfig.ProxyHeader = serverConf.ForwardedIPHeader
	}

	server := fiber.New(FiberServerConfig)
	server.Use(recover.New())
	server.Use(compress.New())
	server.Use(fiberlogger.New(fiberlogger.Config{Output: serverConf.AccessLogWriter}))
	server.Use(requestid.New())

	if serverConf.CORS.Enabled {
		server.Use(cors.New(corsConfigFromConf(serverConf.CORS)))
		log.Info().Msg("CORS enabled for main server")
	}

	return server, nil
}

func initStatsCollector(statsConfig apistats.Config, storages model.Backends, server *fiber.App) (
	*stats.Collector, error,
) {
	if !statsConfig.Enabled || storages.Stats == nil {
		return nil, nil
	}

	collector, err := stats.NewCollector(statsConfig, storages.Stats)
	if err != nil {
		log.Warn().Err(err).Msg("failed to initialize stats collector, statistics disabled")
		return nil, nil
	}

	server.Use(collector.Middleware())
	return collector, nil
}

// initStatsAggregator builds the daily-stats aggregator when stats are enabled
// and a stats storage backend is available. Returns nil otherwise; Start/Stop
// treat a nil aggregator as a no-op.
func initStatsAggregator(statsConfig apistats.Config, storages model.Backends) *stats.Aggregator {
	if !statsConfig.Enabled || storages.Stats == nil {
		return nil
	}
	return stats.NewAggregator(
		storages.Stats,
		statsConfig.DetailedRetention,
		statsConfig.AggregatedRetention,
	)
}

func buildDynamicFederationEntity(
	entity *LightHouse, entityID string, storages model.Backends,
) oidfed.FederationEntity {
	return &oidfed.DynamicFederationEntity{
		ID: entityID,
		Metadata: func() (*oidfed.Metadata, error) {
			m, err := storage.GetMetadata(storages.KV)
			if err != nil {
				return nil, err
			}
			// Merge DB metadata with lighthouse's federation metadata.
			// Values from entity.fedMetadata should overwrite existing ones in m.
			if m == nil {
				m = &oidfed.Metadata{}
			}
			// Build base map from existing federation entity metadata (if any)
			var base map[string]any
			if m.FederationEntity != nil {
				var tmp map[string]any
				if bs, err := json.Marshal(m.FederationEntity); err == nil {
					_ = json.Unmarshal(bs, &tmp)
				}
				base = tmp
			} else {
				base = make(map[string]any)
			}
			// Overlay with lighthouse federation metadata (non-zero fields only due to omitempty)
			var overlay map[string]any
			if bs, err := json.Marshal(entity.fedMetadata); err == nil {
				_ = json.Unmarshal(bs, &overlay)
			}

			merged := utils.MergeMaps(true, base, overlay)
			// Unmarshal merged back into FederationEntityMetadata
			var mergedFE oidfed.FederationEntityMetadata
			if bs, err := json.Marshal(merged); err == nil {
				if err := json.Unmarshal(bs, &mergedFE); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
			// Merge Extra maps separately with overwrite
			var extraBase map[string]any
			if m.FederationEntity != nil && m.FederationEntity.Extra != nil {
				extraBase = m.FederationEntity.Extra
			} else {
				extraBase = map[string]any{}
			}
			mergedFE.Extra = utils.MergeMaps(true, extraBase, entity.fedMetadata.Extra)
			m.FederationEntity = &mergedFE
			return m, nil
		},
		AuthorityHints: func() ([]string, error) {
			return storage.GetAuthorityHints(storages.AuthorityHints)
		},
		ConfigurationLifetime: func() (time.Duration, error) {
			return storage.GetEntityConfigurationLifetime(storages.KV)
		},
		EntityStatementSigner: func() (*jwx.EntityStatementSigner, error) {
			return entity.GeneralJWTSigner.EntityStatementSigner(), nil
		},
		TrustMarks: func() ([]*oidfed.EntityConfigurationTrustMarkConfig, error) {
			return entity.trustMarkConfigProvider.GetConfigs()
		},
		TrustMarkIssuers: func() (oidfed.AllowedTrustMarkIssuers, error) {
			return storages.TrustMarkTypes.IssuersByType()
		},
		TrustMarkOwners: func() (oidfed.TrustMarkOwners, error) {
			return storages.TrustMarkTypes.OwnersByType()
		},
		Extra: func() (map[string]any, []string, error) {
			extra, crits, err := storage.GetEntityConfigurationAdditionalClaims(storages.AdditionalClaims)
			if err != nil {
				return nil, nil, err
			}
			if extra == nil {
				extra = make(map[string]any)
			}
			extra["lighthouse_version"] = version.VERSION
			return extra, crits, nil
		},
	}
}

func registerEntityConfigurationEndpoint(server *fiber.App, entity *LightHouse) {
	server.Get(
		oidfedconst.FederationSuffix, func(ctx *fiber.Ctx) error {
			var cached []byte
			set, err := cache.Get(internal.CacheKeyEntityConfiguration, &cached)
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			if set {
				ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeEntityStatement)
				return ctx.Send(cached)
			}
			ec, err := entity.EntityConfigurationPayload()
			if err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			jwt, err := entity.SignEntityStatement(*ec)
			if err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if cacheErr := cache.Set(
				internal.CacheKeyEntityConfiguration, jwt,
				min(MaximumEntityConfigurationCachePeriod, time.Until(ec.ExpiresAt.Time.Add(-1*time.Minute))),
			); cacheErr != nil {
				log.Error().Err(cacheErr).Msg("failed to cache entity configuration")
			}
			ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeEntityStatement)
			return ctx.Send(jwt)
		},
	)
}

func initAdminAPI(
	admin AdminAPIOptions,
	serverConf ServerConf,
	server *fiber.App,
	entity *LightHouse,
	entityID string,
	storages model.Backends,
	fedEntity oidfed.FederationEntity,
	keyManagement adminapi.KeyManagement,
	trustMarkConfigProvider *storage.TrustMarkConfigProvider,
) (
	*fiber.App,
	error,
) {
	if !admin.Enabled {
		return nil, nil
	}

	var adminAPIServer *fiber.App
	if admin.Port > 0 && admin.Port != serverConf.Port {
		adminAPIServer = fiber.New(FiberServerConfig)
		adminAPIServer.Use(recover.New())
		adminAPIServer.Use(compress.New())
		adminAPIServer.Use(fiberlogger.New(fiberlogger.Config{Output: serverConf.AccessLogWriter}))
		adminAPIServer.Use(requestid.New())

		if admin.CORS.Enabled {
			adminAPIServer.Use(cors.New(corsConfigFromConf(admin.CORS)))
			log.Info().Msg("CORS enabled for admin API server")
		}
	} else {
		adminAPIServer = server
	}

	adminGroup := adminAPIServer.Group("/api/v1/admin")

	if admin.CORS.Enabled && adminAPIServer == server && !serverConf.CORS.Enabled {
		adminGroup.Use(cors.New(corsConfigFromConf(admin.CORS)))
		log.Info().Msg("CORS enabled for admin API routes on main server")
	}

	err := adminapi.Register(
		adminGroup, entityID, storages,
		fedEntity,
		keyManagement,
		entity,
		&adminapi.Options{
			UsersEnabled:               admin.UsersEnabled,
			Port:                       admin.Port,
			TrustMarkConfigInvalidator: trustMarkConfigProvider,
			Actor: adminapi.ActorConfig{
				Header: admin.ActorHeader,
				Source: adminapi.ActorSource(admin.ActorSource),
			},
		},
	)
	if err != nil {
		return nil, err
	}

	return adminAPIServer, nil
}

// HttpHandlerFunc returns an http.HandlerFunc for serving all the necessary endpoints
func (fed *LightHouse) HttpHandlerFunc() http.HandlerFunc {
	return adaptor.FiberApp(fed.server)
}

// Listen starts an http server at the specific address for serving all the
// necessary endpoints
func (fed *LightHouse) Listen(addr string) error {
	return fed.server.Listen(addr)
}

// SetJTICleanupStop sets the cleanup stop function
func (fed *LightHouse) SetJTICleanupStop(stop func()) {
	fed.jtiCleanupStop = stop
}

// TrustAnchorRepo returns the trust anchor repository, or nil if not initialized.
func (fed *LightHouse) TrustAnchorRepo() *TrustAnchorRepo {
	return fed.trustAnchorRepo
}

// SetTrustAnchorRepo sets the trust anchor repository.
func (fed *LightHouse) SetTrustAnchorRepo(repo *TrustAnchorRepo) {
	fed.trustAnchorRepo = repo
}

// TAJWKSRefresher returns the TA JWKS refresher, or nil if not initialized.
func (fed *LightHouse) TAJWKSRefresher() *oidfed.TAJWKSRefresher {
	return fed.taJWKSRefresher
}

// SetTAJWKSRefresher sets the TA JWKS refresher.
func (fed *LightHouse) SetTAJWKSRefresher(r *oidfed.TAJWKSRefresher) {
	fed.taJWKSRefresher = r
}

// SubordinateJWKSRefresher returns the subordinate JWKS refresher, or nil if
// not initialized.
func (fed *LightHouse) SubordinateJWKSRefresher() *oidfed.SubordinateJWKSRefresher {
	return fed.subordinateJWKSRefresher
}

// SetSubordinateJWKSRefresher sets the subordinate JWKS refresher.
func (fed *LightHouse) SetSubordinateJWKSRefresher(r *oidfed.SubordinateJWKSRefresher) {
	fed.subordinateJWKSRefresher = r
}

// SyncTrustAnchor reloads a TA from the database and updates the in-memory
// repository. Called by the admin API after a DB mutation on a trust anchor.
func (fed *LightHouse) SyncTrustAnchor(entityID string) {
	if fed.trustAnchorRepo != nil {
		fed.trustAnchorRepo.AddOrUpdate(entityID)
	}
}

// RemoveTrustAnchor removes a TA from the in-memory repository.
// Called by the admin API after a trust anchor is deleted.
func (fed *LightHouse) RemoveTrustAnchor(entityID string) {
	if fed.trustAnchorRepo != nil {
		fed.trustAnchorRepo.Remove(entityID)
	}
}

// TAResolver returns a middleware.TAResolver that resolves trust anchor entity
// IDs to oidfed.TrustAnchors via the in-memory repo. If the repo is nil, falls
// back to creating TrustAnchors from entity IDs without JWKS (suitable for
// trust-chain resolution that fetches JWKS itself).
func (fed *LightHouse) TAResolver() middleware.TAResolver {
	return func(entityIDs []string) oidfed.TrustAnchors {
		if fed.trustAnchorRepo != nil {
			return fed.trustAnchorRepo.Resolve(entityIDs...)
		}
		return oidfed.NewTrustAnchorsFromEntityIDs(entityIDs...)
	}
}

//go:embed banner.txt
var bannerTxt string

func (fed *LightHouse) banner() {
	bannerWidth := 0
	if fed.LogoBanner {
		bannerWidth = 104
		fmt.Println(bannerTxt)
	}
	if fed.VersionBanner {
		fmt.Println(version.Banner(bannerWidth))
	} else {
		log.Info().Str("version", version.VERSION).Msg("Starting lighthouse")
	}
}

func (fed *LightHouse) Start() {
	fed.banner()

	// Start stats collector if enabled
	if fed.statsCollector != nil {
		fed.statsCollector.Start()
	}

	// Start the daily-stats aggregator if enabled. Only run in the parent
	// process when prefork is enabled to avoid multiple aggregators racing on
	// the same database.
	if fed.statsAggregator != nil && !fiber.IsChild() {
		ctx, cancel := context.WithCancel(context.Background())
		fed.statsAggregatorCancel = cancel
		go func() {
			if err := fed.statsAggregator.Run(ctx); err != nil &&
				err != context.Canceled {
				log.Warn().Err(err).Msg("stats aggregator stopped with error")
			}
		}()
	}

	conf := fed.serverConf
	adminTLS := fed.adminAPIServer != nil && fed.adminAPIServer != fed.server && fed.serverConf.AdminTLS.Enabled

	// Admin API server
	if fed.adminAPIServer != nil && fed.adminAPIServer != fed.server {
		if adminTLS {
			log.Info().Int("port", conf.AdminAPIPort).Msg("starting admin api server with TLS")
			go func() {
				if err := fed.adminAPIServer.ListenTLS(
					fmt.Sprintf("%s:%d", conf.IPListen, conf.AdminAPIPort),
					fed.serverConf.AdminTLS.Cert,
					fed.serverConf.AdminTLS.Key,
				); err != nil {
					log.Fatal().Err(err).Send()
				}
			}()
		} else {
			log.Info().Int("port", conf.AdminAPIPort).Msg("starting admin api server")
			go func() {
				if err := fed.adminAPIServer.Listen(
					fmt.Sprintf("%s:%d", conf.IPListen, conf.AdminAPIPort),
				); err != nil {
					log.Fatal().Err(err).Send()
				}
			}()
		}
	}
	if !conf.TLS.Enabled {
		log.Info().Int("port", conf.Port).Msg("TLS is disabled starting http server")
		if err := fed.server.Listen(fmt.Sprintf("%s:%d", conf.IPListen, conf.Port)); err != nil {
			log.Fatal().Err(err).Send()
		}
		return
	}
	// TLS enabled
	if conf.TLS.RedirectHTTP {
		// HTTP redirect server only needs to run in one process
		httpServer := fiber.New(FiberServerConfig)
		httpServer.All(
			"*", func(ctx *fiber.Ctx) error {
				//goland:noinspection HttpUrlsUsage
				return ctx.Redirect(
					strings.Replace(ctx.Request().URI().String(), "http://", "https://", 1),
					fiber.StatusPermanentRedirect,
				)
			},
		)
		log.Info().Msg("TLS and http redirect enabled, starting redirect server on port 80")
		go func() {
			if err := httpServer.Listen(conf.IPListen + ":80"); err != nil {
				log.Fatal().Err(err).Send()
			}
		}()
	}
	time.Sleep(time.Millisecond) // This is just for a more pretty output with the tls header printed after the http one
	log.Info().Msg("TLS enabled, starting https server on port 443")
	if err := fed.server.ListenTLS(conf.IPListen+":443", conf.TLS.Cert, conf.TLS.Key); err != nil {
		log.Fatal().Err(err).Send()
	}
}

// Stop gracefully shuts down the LightHouse server and its components.
func (fed *LightHouse) Stop() error {
	// Stop background services
	fed.stopBackgroundServices()

	// Stop TA JWKS refresher if running
	if fed.taJWKSRefresher != nil {
		fed.taJWKSRefresher.Stop()
	}

	// Stop subordinate JWKS refresher if running
	if fed.subordinateJWKSRefresher != nil {
		fed.subordinateJWKSRefresher.Stop()
	}

	// Stop JTI cleanup if running
	if fed.jtiCleanupStop != nil {
		fed.jtiCleanupStop()
	}

	// Stop stats collector if running
	if fed.statsCollector != nil {
		if err := fed.statsCollector.Stop(); err != nil {
			log.Warn().Err(err).Msg("error stopping stats collector")
		}
	}

	// Stop stats aggregator if running
	if fed.statsAggregatorCancel != nil {
		fed.statsAggregatorCancel()
	}

	// Shutdown fiber servers
	if err := fed.server.Shutdown(); err != nil {
		return err
	}

	if fed.adminAPIServer != nil && fed.adminAPIServer != fed.server {
		if err := fed.adminAPIServer.Shutdown(); err != nil {
			return err
		}
	}

	return nil
}

// CreateSubordinateStatement returns an oidfed.EntityStatementPayload for the passed storage.ExtendedSubordinateInfo
func (fed *LightHouse) CreateSubordinateStatement(subordinate *model.ExtendedSubordinateInfo) oidfed.EntityStatementPayload {
	now := time.Now()
	lifetime, err := storage.GetSubordinateStatementLifetime(fed.storages.KV)
	if err != nil {
		log.Warn().Err(err).Msg("failed to get subordinate statement lifetime, using default")
		lifetime = storage.DefaultSubordinateStatementLifetime
	}

	// Build extra claims and critical extensions from subordinate additional claims
	// (includes fallback from general claims applied in storage layer)
	extra := make(map[string]any)
	var criticalExtensions []string
	for _, claim := range subordinate.SubordinateAdditionalClaims {
		extra[claim.Claim] = claim.Value
		if claim.Crit {
			criticalExtensions = append(criticalExtensions, claim.Claim)
		}
	}

	// Load metadata policy crit from KV store (global setting for all subordinates)
	// and filter to only include operators that are actually used in the metadata policy
	var configuredCritOperators []oidfed.PolicyOperatorName
	if _, err := fed.storages.KV.GetAs(
		model.KeyValueScopeSubordinateStatement,
		model.KeyValueKeyMetadataPolicyCrit,
		&configuredCritOperators,
	); err != nil {
		log.Warn().Err(err).Msg("failed to get metadata policy crit from KV store")
	}

	// Filter to only include operators actually used in the metadata policy
	metadataPolicyCrit := filterUsedOperators(subordinate.MetadataPolicy, configuredCritOperators)

	// Publish only non-expired keys: a key is dropped if it has an `exp` claim
	// strictly before now (exp == now is still valid). Keys without `exp` are
	// always included. The stored JWKS is not modified, so incoming Entity
	// Configurations / signed JWK Sets can still be verified against historical
	// keys during rotation.
	filteredJWKS := subordinate.JWKS.Keys.WithoutExpired(now)
	if subordinate.JWKS.Keys.Set != nil && subordinate.JWKS.Keys.Len() > 0 && filteredJWKS.Len() == 0 {
		log.Warn().Str("subordinate", subordinate.EntityID).
			Msg("all subordinate keys are expired; publishing empty JWKS in subordinate statement")
	}

	// Cap the statement's expiration to the latest key expiration so the
	// statement never outlives any published key. No cap is applied when no
	// published key has an `exp` (MaximalExpirationTime returns zero).
	exp := unixtime.Unixtime{Time: now.Add(lifetime)}
	if maxKeyExp := filteredJWKS.MaximalExpirationTime(); !maxKeyExp.IsZero() && maxKeyExp.Before(exp.Time) {
		exp = unixtime.Unixtime{Time: maxKeyExp.Time}
	}

	return oidfed.EntityStatementPayload{
		Issuer:             fed.FederationEntity.EntityID(),
		Subject:            subordinate.EntityID,
		IssuedAt:           unixtime.Unixtime{Time: now},
		ExpiresAt:          exp,
		SourceEndpoint:     fed.fedMetadata.FederationFetchEndpoint,
		JWKS:               filteredJWKS,
		Metadata:           subordinate.Metadata,
		MetadataPolicy:     subordinate.MetadataPolicy,
		Constraints:        subordinate.Constraints,
		CriticalExtensions: criticalExtensions,
		MetadataPolicyCrit: metadataPolicyCrit,
		Extra:              extra,
	}
}

// subordinateStatementCacheTTL computes the cache TTL for a signed subordinate
// statement, ensuring the cache entry expires before the statement itself and
// no later than the earliest published key expiration.
// Returns a non-positive duration if the entry should not be cached.
func subordinateStatementCacheTTL(payload oidfed.EntityStatementPayload) time.Duration {
	ttl := MaximumSubordinateStatementCachePeriod
	if t := time.Until(payload.ExpiresAt.Time.Add(-time.Minute)); t < ttl {
		ttl = t
	}
	if minKeyExp := payload.JWKS.MinimalExpirationTime(); !minKeyExp.IsZero() {
		if t := time.Until(minKeyExp.Time); t < ttl {
			ttl = t
		}
	}
	return ttl
}

// filterUsedOperators returns only the operators from configuredCrit that are actually
// used in the given metadata policy.
func filterUsedOperators(
	mp *oidfed.MetadataPolicies, configuredCrit []oidfed.PolicyOperatorName,
) []oidfed.PolicyOperatorName {
	if mp == nil || len(configuredCrit) == 0 {
		return nil
	}

	// Collect all operators used in the metadata policy
	usedOperators := make(map[oidfed.PolicyOperatorName]bool)
	collectOperatorsFromPolicy := func(policy oidfed.MetadataPolicy) {
		if policy == nil {
			return
		}
		for _, entry := range policy {
			for op := range entry {
				usedOperators[op] = true
			}
		}
	}

	// Check all standard entity type policies
	collectOperatorsFromPolicy(mp.OpenIDProvider)
	collectOperatorsFromPolicy(mp.RelyingParty)
	collectOperatorsFromPolicy(mp.OAuthAuthorizationServer)
	collectOperatorsFromPolicy(mp.OAuthClient)
	collectOperatorsFromPolicy(mp.OAuthProtectedResource)
	collectOperatorsFromPolicy(mp.FederationEntity)

	// Check extra policies
	for _, policy := range mp.Extra {
		collectOperatorsFromPolicy(policy)
	}

	// Filter configured crit operators to only those actually used
	var result []oidfed.PolicyOperatorName
	for _, op := range configuredCrit {
		if usedOperators[op] {
			result = append(result, op)
		}
	}
	return result
}

// AdminAPIOptions controls initialization of the admin API.
type AdminAPIOptions struct {
	Enabled      bool
	UsersEnabled bool
	// Port: 0 mounts on main server under /api/v1/admin; >0 starts a separate server on this port
	Port int
	// ActorHeader is the HTTP header name to extract the actor from for event history.
	// Default: "X-Actor"
	ActorHeader string
	// ActorSource is the preferred source for actor extraction ("basic_auth" or "header").
	// Default: "basic_auth" (tries basic auth username first, then falls back to header)
	ActorSource string
	// CORS holds CORS middleware configuration for the admin API.
	CORS CORSConf
	// TLS holds TLS configuration for the admin API.
	TLS TLSConf
}

// corsConfigFromConf converts a CORSConf to a Fiber CORS middleware configuration.
func corsConfigFromConf(conf CORSConf) cors.Config {
	return cors.Config{
		AllowOrigins:     conf.AllowOrigins,
		AllowMethods:     conf.AllowMethods,
		AllowHeaders:     conf.AllowHeaders,
		AllowCredentials: conf.AllowCredentials,
		ExposeHeaders:    conf.ExposeHeaders,
		MaxAge:           conf.MaxAge,
	}
}
