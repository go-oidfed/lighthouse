package main

import (
	"os"
	"strings"
	"time"

	"github.com/go-oidfed/lib/cache"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse"
	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
	"github.com/go-oidfed/lighthouse/internal/logger"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
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

	if err := initCache(&c.Caching); err != nil {
		log.WithError(err).Fatal("failed to initialize cache")
	}

	backs, err := initStorage(&c.Storage, c.API.Admin.Argon2idParams)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize storage")
	}

	logStorageWarnings(c.Server, &c.Storage, &c.Caching)

	statsOpts := buildStatsOptions(&c.Stats)

	if c.Stats.Enabled {
		if err := storage.MigrateStatsFromBackends(backs); err != nil {
			log.WithError(err).Warn("failed to migrate stats tables")
		}
	}

	lh, err := initLighthouse(&c, backs, statsOpts)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize lighthouse")
	}

	setupTrustMarkIssuer(lh, c.EntityID, &backs)

	log.Info("Initialized Entity")

	proactiveResolver, err := registerEndpoints(lh, &c, &backs)
	if err != nil {
		log.WithError(err).Fatal("failed to register endpoints")
	}

	log.Info("Added Endpoints")

	if err = startBackgroundServices(proactiveResolver, &c); err != nil {
		log.WithError(err).Fatal("failed to start background services")
	}

	lh.Start()
}

func initCache(caching *config.CachingConf) error {
	if caching.Disabled {
		cache.UseNoopCache()
		return nil
	}

	if redisAddr := caching.RedisAddr; redisAddr != "" {
		if err := cache.UseRedisCache(
			&redis.Options{
				Addr:     redisAddr,
				Username: caching.Username,
				Password: caching.Password,
				DB:       caching.RedisDB,
			},
		); err != nil {
			return err
		}
		log.Info("Loaded Redis Cache")
	}

	if caching.MaxLifetime.Duration() != 0 {
		cache.SetMaxLifetime(caching.MaxLifetime.Duration())
	}

	return nil
}

func initStorage(storageConf *config.StorageConf, usersHash storage.Argon2idParams) (model.Backends, error) {
	cfg := storage.Config{
		Driver:    storageConf.Driver,
		DSN:       storageConf.DSN,
		DataDir:   storageConf.DataDir,
		Debug:     storageConf.Debug,
		UsersHash: usersHash,
	}
	return storage.LoadStorageBackends(cfg)
}

func logStorageWarnings(server lighthouse.ServerConf, storageConf *config.StorageConf, caching *config.CachingConf) {
	if server.Prefork && storageConf.Driver == "sqlite" {
		log.Warn(
			"Using SQLite with prefork enabled may cause write conflicts. " +
				"Consider using MySQL or PostgreSQL for production deployments with prefork.",
		)
	}

	if server.Prefork && caching.RedisAddr == "" && !caching.Disabled {
		log.Warn(
			"Prefork is enabled without Redis cache. In-memory caches will be process-local " +
				"and may lead to inconsistencies. It is strongly recommended to configure Redis " +
				"for caching when using prefork mode.",
		)
	}
}

func buildStatsOptions(stats *config.StatsConf) lighthouse.StatsOptions {
	return lighthouse.StatsOptions{
		Enabled:             stats.Enabled,
		BufferSize:          stats.Buffer.Size,
		FlushInterval:       stats.Buffer.FlushInterval,
		FlushThreshold:      stats.Buffer.FlushThreshold,
		CaptureClientIP:     stats.Capture.ClientIP,
		CaptureUserAgent:    stats.Capture.UserAgent,
		CaptureQueryParams:  stats.Capture.QueryParams,
		GeoIPEnabled:        stats.Capture.GeoIP.Enabled,
		GeoIPDBPath:         stats.Capture.GeoIP.DatabasePath,
		DetailedRetention:   stats.DetailedRetention(),
		AggregatedRetention: stats.AggregatedRetention(),
		Endpoints:           stats.Endpoints,
	}
}

func initLighthouse(c *config.Config, backs model.Backends, statsOpts lighthouse.StatsOptions) (
	*lighthouse.LightHouse, error,
) {
	lh, err := lighthouse.NewLightHouse(
		c.Server,
		c.EntityID,
		c.Signing.SigningConf,
		backs,
		lighthouse.AdminAPIOptions{
			Enabled:      c.API.Admin.Enabled,
			UsersEnabled: c.API.Admin.UsersEnabled,
			Port:         c.API.Admin.Port,
			ActorHeader:  c.API.Admin.ActorHeader,
			ActorSource:  c.API.Admin.ActorSource,
			CORS:         c.API.Admin.CORS,
		},
		statsOpts,
	)
	if err != nil {
		return nil, err
	}

	lh.LogoBanner = c.Logging.Banner.Logo
	lh.VersionBanner = c.Logging.Banner.Version

	return lh, nil
}

func setupTrustMarkIssuer(lh *lighthouse.LightHouse, entityID string, backs *model.Backends) {
	lh.TrustMarkIssuer = oidfed.NewTrustMarkIssuer(
		entityID, lh.GeneralJWTSigner.TrustMarkSigner(),
		nil,
	)

	if backs.TrustMarkSpecs != nil {
		dbProvider := lighthouse.NewDBTrustMarkSpecProvider(backs.TrustMarkSpecs)
		lh.TrustMarkIssuer.SetProvider(dbProvider)
		log.Info("Configured DB-based TrustMarkSpecProvider")
	}
}

func registerEndpoints(lh *lighthouse.LightHouse, c *config.Config, backs *model.Backends) (
	*oidfed.ProactiveResolver, error,
) {
	var proactiveResolver *oidfed.ProactiveResolver

	if endpoint := c.Endpoints.FetchEndpoint; endpoint.IsSet() {
		lh.AddFetchEndpoint(endpoint, backs.Subordinates)
	}

	if endpoint := c.Endpoints.ListEndpoint; endpoint.IsSet() {
		lh.AddSubordinateListingEndpoint(endpoint, backs.Subordinates, backs.TrustMarks)
	}

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
		eligibilityCache := lighthouse.NewEligibilityCache()
		stopEligibilityCacheCleanup := eligibilityCache.StartCleanupRoutine(5 * time.Minute)
		defer stopEligibilityCacheCleanup()

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
			var err error
			checker, err = lighthouse.EntityCheckerFromEntityCheckerConfig(checkerConfig)
			if err != nil {
				return nil, err
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
			collector = pec
		}
		lh.AddEntityCollectionEndpoint(
			endpoint.EndpointConf, collector, endpoint.AllowedTrustAnchors, endpoint.PaginationLimit > 0,
		)
	}

	return proactiveResolver, nil
}

func startBackgroundServices(proactiveResolver *oidfed.ProactiveResolver, c *config.Config) error {
	if proactiveResolver != nil && !fiber.IsChild() {
		proactiveResolver.Start()
	}

	if endpoint := c.Endpoints.EntityCollectionEndpoint; endpoint.IsSet() && endpoint.Interval.Duration() != 0 {
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
		if !fiber.IsChild() {
			pec.Start()
		}
	}

	return nil
}
