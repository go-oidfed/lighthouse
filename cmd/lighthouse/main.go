package main

import (
	"os"
	"time"

	"github.com/go-oidfed/lib/cache"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse"
	"github.com/go-oidfed/lighthouse/api/stats"
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
	log.Info().Msg("Loaded Config")
	c := config.Get()

	if err := initCache(&c.Caching); err != nil {
		log.Fatal().Err(err).Msg("failed to initialize cache")
	}

	backs, err := initStorage(&c.Storage, c.API.Admin.Argon2idParams)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize storage")
	}

	statsOpts := c.Stats.ToAPIConfig()

	if c.Stats.Enabled {
		if err = storage.MigrateStatsFromBackends(backs); err != nil {
			log.Warn().Err(err).Msg("failed to migrate stats tables")
		}
	}

	lh, err := initLighthouse(&c, backs, statsOpts)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize lighthouse")
	}

	setupTrustMarkIssuer(lh, c.EntityID, &backs)

	log.Info().Msg("Initialized Entity")

	// Build the trust anchor repository.
	// The JWKS refresher is started after the repo is loaded.
	if err = setupTrustAnchorRepo(lh, &backs); err != nil {
		log.Fatal().Err(err).Msg("failed to setup trust anchor repository")
	}
	if err = startTAJWKSRefresher(lh, &backs); err != nil {
		log.Fatal().Err(err).Msg("failed to start TA JWKS refresher")
	}

	// Load federation endpoints from the database.
	if err = lh.LoadEndpointsFromDB(); err != nil {
		log.Fatal().Err(err).Msg("failed to load endpoints from DB")
	}

	if err = startSubordinateJWKSRefresher(lh, &backs); err != nil {
		log.Fatal().Err(err).Msg("failed to start subordinate JWKS refresher")
	}

	log.Info().Msg("Added Endpoints")

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
		log.Info().Msg("Loaded Redis Cache")
	}

	if caching.MaxLifetime.Duration() != 0 {
		cache.SetMaxLifetime(caching.MaxLifetime.Duration())
	}

	return nil
}

func initStorage(
	storageConf *config.StorageConf, usersHash storage.Argon2idParams,
) (model.Backends, error) {
	cfg := storage.Config{
		Driver:         storageConf.Driver,
		DSN:            storageConf.DSN,
		DataDir:        storageConf.DataDir,
		Debug:          storageConf.Debug,
		UsersHash:      usersHash,
		JTIStorageType: storageConf.EndpointAuth.JTIBackend,
	}
	return storage.LoadStorageBackends(cfg)
}

func initLighthouse(c *config.Config, backs model.Backends, statsConfig stats.Config) (
	*lighthouse.LightHouse, error,
) {
	c.Server.AdminTLS = c.API.Admin.TLS
	c.Server.AccessLogWriter = logger.AccessLogger()

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
			TLS:          c.API.Admin.TLS,
		},
		statsConfig,
	)
	if err != nil {
		return nil, err
	}

	// Start JTI cleanup goroutine if using DB backend
	if c.Storage.EndpointAuth.JTIBackend == storage.JTIStorageDB {
		lh.SetJTICleanupStop(startJTICleanup(backs.JTI, c.Storage.EndpointAuth.JTICleanupInterval.Duration()))
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
		log.Info().Msg("Configured DB-based TrustMarkSpecProvider")
	}
}

// setupTrustAnchorRepo builds the trust anchor repository from the database.
// Called in all processes (read-only cache).
func setupTrustAnchorRepo(lh *lighthouse.LightHouse, backs *model.Backends) error {
	if backs.TrustAnchors == nil {
		log.Warn().Msg("Trust anchor storage not available; skipping TA repository setup")
		return nil
	}
	repo := lighthouse.NewTrustAnchorRepo(backs.TrustAnchors)
	if err := repo.Load(); err != nil {
		return errors.Wrap(err, "failed to load trust anchor repository")
	}
	log.Info().Int("count", repo.Count()).Msg("Loaded trust anchor repository")
	lh.SetTrustAnchorRepo(repo)
	return nil
}

// startTAJWKSRefresher starts the TA JWKS refresher.
func startTAJWKSRefresher(lh *lighthouse.LightHouse, backs *model.Backends) error {
	repo := lh.TrustAnchorRepo()
	if repo == nil || len(repo.AllWithJWKSUpdate()) == 0 {
		log.Debug().Msg("No trust anchors with enable_jwks_update=true; skipping refresher")
		return nil
	}
	dbJWKStorage := storage.NewDBJWKStorage(storage.NewTrustAnchorStorage(backs.DB))
	refresher, err := lighthouse.SetupTAJWKSRefresher(repo, dbJWKStorage)
	if err != nil {
		return errors.Wrap(err, "failed to start TA JWKS refresher")
	}
	lh.SetTAJWKSRefresher(refresher)
	return nil
}

// startSubordinateJWKSRefresher starts the subordinate JWKS refresher if
// subordinate storage is available and any subordinates have
// enable_jwks_update=true.
func startSubordinateJWKSRefresher(lh *lighthouse.LightHouse, backs *model.Backends) error {
	if backs.Subordinates == nil {
		log.Debug().Msg("Subordinate storage not available; skipping subordinate JWKS refresher")
		return nil
	}
	refresher, err := lighthouse.SetupSubordinateJWKSRefresher(backs.Subordinates, backs.SubordinateEvents)
	if err != nil {
		return errors.Wrap(err, "failed to start subordinate JWKS refresher")
	}
	lh.SetSubordinateJWKSRefresher(refresher)
	return nil
}

func startJTICleanup(jtiStorage model.JTIStorageBackend, interval time.Duration) func() {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := jtiStorage.Cleanup(); err != nil {
					log.Warn().Err(err).Msg("JTI cleanup failed")
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return func() { close(done) }
}
