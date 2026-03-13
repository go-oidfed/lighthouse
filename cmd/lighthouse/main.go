package main

import (
	"os"
	"strings"
	"time"

	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/cache"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/go-oidfed/lighthouse"
	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
	"github.com/go-oidfed/lighthouse/internal/logger"
	"github.com/go-oidfed/lighthouse/storage"
)

func main() {
	var configFile string
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}
	config.MustLoad(configFile)
	logger.Init()
	log.Info("Loaded Config")
	c := config.Get()

	if c.Caching.Disabled {
		cache.UseNoopCache()
	} else if redisAddr := c.Caching.RedisAddr; redisAddr != "" {
		if err := cache.UseRedisCache(
			&redis.Options{
				Addr:     redisAddr,
				Username: c.Caching.Username,
				Password: c.Caching.Password,
				DB:       c.Caching.RedisDB,
			},
		); err != nil {
			log.WithError(err).Fatal("could not init redis cache")
		}
		log.Info("Loaded Redis Cache")
	}
	if c.Caching.MaxLifetime.Duration() != 0 {
		cache.SetMaxLifetime(c.Caching.MaxLifetime.Duration())
	}

	// Build storage with user hash params coming from api.admin.users_hash
	backs, err := storage.LoadStorageBackends(
		storage.Config{
			Driver:    c.Storage.Driver,
			DSN:       c.Storage.DSN,
			DataDir:   c.Storage.DataDir,
			Debug:     c.Storage.Debug,
			UsersHash: c.API.Admin.Argon2idParams,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	// Warn about SQLite + Prefork combination
	if c.Server.Prefork && c.Storage.Driver == "sqlite" {
		log.Warn(
			"Using SQLite with prefork enabled may cause write conflicts. " +
				"Consider using MySQL or PostgreSQL for production deployments with prefork.",
		)
	}

	// Warn about prefork without Redis cache
	if c.Server.Prefork && c.Caching.RedisAddr == "" && !c.Caching.Disabled {
		log.Warn(
			"Prefork is enabled without Redis cache. In-memory caches will be process-local " +
				"and may lead to inconsistencies. It is strongly recommended to configure Redis " +
				"for caching when using prefork mode.",
		)
	}

	signingConf := c.Signing

	// for _, tmc := range c.Federation.TrustMarks {
	// 	if err = tmc.Verify(
	// 		c.EntityID, c.Endpoints.TrustMarkEndpoint.ValidateURL(c.EntityID),
	// 		jwx.NewTrustMarkSigner(keys.Federation()),
	// 	); err != nil {
	// 		log.Fatal(err)
	// 	}
	// }

	// Build stats options from config
	statsOpts := lighthouse.StatsOptions{
		Enabled:             c.Stats.Enabled,
		BufferSize:          c.Stats.Buffer.Size,
		FlushInterval:       c.Stats.Buffer.FlushInterval,
		FlushThreshold:      c.Stats.Buffer.FlushThreshold,
		CaptureClientIP:     c.Stats.Capture.ClientIP,
		CaptureUserAgent:    c.Stats.Capture.UserAgent,
		CaptureQueryParams:  c.Stats.Capture.QueryParams,
		GeoIPEnabled:        c.Stats.Capture.GeoIP.Enabled,
		GeoIPDBPath:         c.Stats.Capture.GeoIP.DatabasePath,
		DetailedRetention:   c.Stats.DetailedRetention(),
		AggregatedRetention: c.Stats.AggregatedRetention(),
		Endpoints:           c.Stats.Endpoints,
	}

	// Migrate stats tables if stats is enabled
	if c.Stats.Enabled {
		if err := storage.MigrateStatsFromBackends(backs); err != nil {
			log.WithError(err).Warn("failed to migrate stats tables")
		}
	}

	lh, err := lighthouse.NewLightHouse(
		config.Get().Server,
		c.EntityID,
		signingConf.SigningConf,
		backs,
		lighthouse.AdminAPIOptions{
			Enabled:      c.API.Admin.Enabled,
			UsersEnabled: c.API.Admin.UsersEnabled,
			Port:         c.API.Admin.Port,
			ActorHeader:  c.API.Admin.ActorHeader,
			ActorSource:  c.API.Admin.ActorSource,
		},
		statsOpts,
	)
	if err != nil {
		panic(err)
	}
	lh.LogoBanner = c.Logging.Banner.Logo
	lh.VersionBanner = c.Logging.Banner.Version

	// lh.Constraints = c.Federation.Constraints
	// lh.CriticalExtensions = c.Federation.CriticalExtensions
	// lh.MetadataPolicyCrit = c.Federation.MetadataPolicyCrit
	// lh.TrustMarks = c.Federation.TrustMarks
	// lh.TrustMarkIssuers = c.Federation.TrustMarkIssuers
	// lh.TrustMarkOwners = c.Federation.TrustMarkOwners

	// Initialize TrustMarkIssuer - specs are loaded dynamically from DB via provider
	lh.TrustMarkIssuer = oidfed.NewTrustMarkIssuer(
		c.EntityID, lh.GeneralJWTSigner.TrustMarkSigner(),
		nil, // No static specs - all loaded from DB
	)

	// Set up DB-based TrustMarkSpecProvider for dynamic trust mark specs
	// This allows trust marks created via the admin API to be issued
	if backs.TrustMarkSpecs != nil {
		dbProvider := lighthouse.NewDBTrustMarkSpecProvider(backs.TrustMarkSpecs)
		lh.TrustMarkIssuer.SetProvider(dbProvider)
		log.Info("Configured DB-based TrustMarkSpecProvider")
	}

	log.Println("Initialized Entity")

	if endpoint := c.Endpoints.FetchEndpoint; endpoint.IsSet() {
		lh.AddFetchEndpoint(endpoint, backs.Subordinates)
	}
	if endpoint := c.Endpoints.ListEndpoint; endpoint.IsSet() {
		lh.AddSubordinateListingEndpoint(endpoint, backs.Subordinates, backs.TrustMarks)
	}
	var proactiveResolver *oidfed.ProactiveResolver
	if endpoint := c.Endpoints.ResolveEndpoint; endpoint.IsSet() {
		if endpoint.ProactiveResolver.Enabled {
			proactiveResolver = &oidfed.ProactiveResolver{
				EntityID: c.EntityID,
				Store: oidfed.ResolveStore{
					BaseDir:   endpoint.ProactiveResolver.ResponseStorage.Dir,
					StoreJWT:  endpoint.ProactiveResolver.ResponseStorage.StoreJWT,
					StoreJSON: endpoint.ProactiveResolver.ResponseStorage.StoreJSON,
				},
				Signer:      lh.ResolveResponseSigner(),
				RefreshLead: endpoint.GracePeriod.Duration(),
				Concurrency: endpoint.ProactiveResolver.ConcurrencyLimit,
				QueueSize:   endpoint.ProactiveResolver.QueueSize,
			}
			// In prefork mode, only start background resolver in parent process
			// to avoid duplicate resolution work across child processes
			if !fiber.IsChild() {
				proactiveResolver.Start()
			}
		}
		lh.AddResolveEndpoint(endpoint.EndpointConf, endpoint.AllowedTrustAnchors, proactiveResolver)
	}
	if endpoint := c.Endpoints.TrustMarkStatusEndpoint; endpoint.IsSet() {
		lh.AddTrustMarkStatusEndpoint(
			endpoint, lighthouse.TrustMarkStatusConfig{
				InstanceStore: backs.TrustMarkInstances,
			},
		)
	}
	if endpoint := c.Endpoints.TrustMarkedEntitiesListingEndpoint; endpoint.IsSet() {
		lh.AddTrustMarkedEntitiesListingEndpoint(endpoint, backs.TrustMarkInstances)
	}
	if endpoint := c.Endpoints.TrustMarkEndpoint; endpoint.IsSet() {
		// Initialize eligibility cache for trust mark issuance
		eligibilityCache := lighthouse.NewEligibilityCache()
		// Start cleanup routine (every 5 minutes)
		stopEligibilityCacheCleanup := eligibilityCache.StartCleanupRoutine(5 * time.Minute)
		defer stopEligibilityCacheCleanup()

		// Initialize issued trust mark cache (TTL is configured per-spec via admin API)
		issuedTrustMarkCache := lighthouse.NewIssuedTrustMarkCache()
		stopIssuedCacheCleanup := issuedTrustMarkCache.StartCleanupRoutine(5 * time.Minute)
		defer stopIssuedCacheCleanup()

		lh.AddTrustMarkEndpointWithConfig(
			endpoint, lighthouse.TrustMarkEndpointConfig{
				Store:                backs.TrustMarks,
				SpecStore:            backs.TrustMarkSpecs,
				InstanceStore:        backs.TrustMarkInstances,
				Cache:                eligibilityCache,
				IssuedTrustMarkCache: issuedTrustMarkCache,
			},
		)
	}
	if endpoint := c.Endpoints.TrustMarkRequestEndpoint; endpoint.IsSet() {
		lh.AddTrustMarkRequestEndpoint(endpoint, backs.TrustMarks)
	}
	if endpoint := c.Endpoints.HistoricalKeysEndpoint; endpoint.IsSet() {
		lh.AddHistoricalKeysEndpoint(endpoint)
	}
	if endpoint := c.Endpoints.EnrollmentEndpoint; endpoint.IsSet() {
		var checker lighthouse.EntityChecker
		if checkerConfig := endpoint.CheckerConfig; checkerConfig.Type != "" {
			checker, err = lighthouse.EntityCheckerFromEntityCheckerConfig(checkerConfig)
			if err != nil {
				panic(err)
			}
		}
		lh.AddEnrollEndpoint(endpoint.EndpointConf, backs.Subordinates, checker)
	}
	if endpoint := c.Endpoints.EnrollmentRequestEndpoint; endpoint.IsSet() {
		lh.AddEnrollRequestEndpoint(endpoint, backs.Subordinates)
	}
	if endpoint := c.Endpoints.EntityCollectionEndpoint; endpoint.IsSet() {
		var collector oidfed.EntityCollector = &oidfed.SimpleEntityCollector{}
		if endpoint.Interval.Duration() != 0 {
			pec := &oidfed.PeriodicEntityCollector{
				TrustAnchors: endpoint.AllowedTrustAnchors,
				Interval:     endpoint.Interval.Duration(),
				Concurrency:  endpoint.ConcurrencyLimit,
			}
			if endpoint.PaginationLimit > 0 {
				pec.SortEntitiesComparisonFunc = func(a, b *oidfed.CollectedEntity) int {
					return strings.Compare(a.EntityID, b.EntityID)
				}
				pec.PagingLimit = endpoint.PaginationLimit
			}
			if proactiveResolver != nil {
				pec.Handler = proactiveResolver
			}
			// In prefork mode, only start background collector in parent process
			// to avoid duplicate collection work across child processes
			if !fiber.IsChild() {
				pec.Start()
			}
			collector = pec
		}
		lh.AddEntityCollectionEndpoint(
			endpoint.EndpointConf, collector, endpoint.AllowedTrustAnchors, endpoint.PaginationLimit > 0,
		)
	}
	log.Info("Added Endpoints")

	lh.Start()
}
