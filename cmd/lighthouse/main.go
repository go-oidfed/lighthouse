package main

import (
	"os"
	"strings"
	"time"

	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/cache"
	"github.com/go-oidfed/lib/jwx"
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
	config.Load(configFile)
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

	signingConf := c.Signing
	signingConf.KeyRotation.EntityConfigurationLifetimeFunc = func() (time.Duration, error) {
		return storage.GetEntityConfigurationLifetime(backs.KV)
	}

	if err = initKey(signingConf, backs.PKStorages); err != nil {
		log.Fatal(err)
	}
	log.Println("Loaded signing key")
	// for _, tmc := range c.Federation.TrustMarks {
	// 	if err = tmc.Verify(
	// 		c.Federation.EntityID, c.Endpoints.TrustMarkEndpoint.ValidateURL(c.Federation.EntityID),
	// 		jwx.NewTrustMarkSigner(keys.Federation()),
	// 	); err != nil {
	// 		log.Fatal(err)
	// 	}
	// }

	lh, err := lighthouse.NewLightHouse(
		config.Get().Server,
		c.Federation.EntityID,
		versatileSigner(), c.Signing.Algorithm,
		lighthouse.SubordinateStatementsConfig{
			MetadataPolicies:             nil,
			SubordinateStatementLifetime: c.Endpoints.FetchEndpoint.StatementLifetime.Duration(),
			// TODO read all of this from config or a storage backend
		},
		backs,
		lighthouse.AdminAPIOptions{
			Enabled:      c.API.Admin.Enabled,
			UsersEnabled: c.API.Admin.UsersEnabled,
			Port:         c.API.Admin.Port,
		},
	)
	if err != nil {
		panic(err)
	}
	lh.LogoBanner = c.Logging.Banner.Logo
	lh.VersionBanner = c.Logging.Banner.Version

	lh.MetadataPolicies = c.Federation.MetadataPolicy
	lh.Constraints = c.Federation.Constraints
	lh.CriticalExtensions = c.Federation.CriticalExtensions
	lh.MetadataPolicyCrit = c.Federation.MetadataPolicyCrit
	// lh.TrustMarks = c.Federation.TrustMarks
	// lh.TrustMarkIssuers = c.Federation.TrustMarkIssuers
	// lh.TrustMarkOwners = c.Federation.TrustMarkOwners

	var trustMarkCheckerMap map[string]lighthouse.EntityChecker
	if specLen := len(c.Endpoints.TrustMarkEndpoint.TrustMarkSpecs); specLen > 0 {
		specs := make([]oidfed.TrustMarkSpec, specLen)
		for i, s := range c.Endpoints.TrustMarkEndpoint.TrustMarkSpecs {
			specs[i] = s.TrustMarkSpec
			if s.CheckerConfig.Type != "" {
				if trustMarkCheckerMap == nil {
					trustMarkCheckerMap = make(map[string]lighthouse.EntityChecker)
				}
				trustMarkCheckerMap[s.TrustMarkType], err = lighthouse.EntityCheckerFromEntityCheckerConfig(
					s.CheckerConfig,
				)
				if err != nil {
					panic(err)
				}
			}
		}
		lh.TrustMarkIssuer = oidfed.NewTrustMarkIssuer(
			c.Federation.EntityID, lh.GeneralJWTSigner.TrustMarkSigner(),
			specs,
		)
	}
	log.Println("Initialized Entity")

	if endpoint := c.Endpoints.FetchEndpoint; endpoint.IsSet() {
		lh.AddFetchEndpoint(endpoint.EndpointConf, backs.Subordinates)
	}
	if endpoint := c.Endpoints.ListEndpoint; endpoint.IsSet() {
		lh.AddSubordinateListingEndpoint(endpoint, backs.Subordinates, backs.TrustMarks)
	}
	var proactiveResolver *oidfed.ProactiveResolver
	if endpoint := c.Endpoints.ResolveEndpoint; endpoint.IsSet() {
		if endpoint.ProactiveResolver.Enabled {
			proactiveResolver = &oidfed.ProactiveResolver{
				EntityID: c.Federation.EntityID,
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
			proactiveResolver.Start()
		}
		lh.AddResolveEndpoint(endpoint.EndpointConf, endpoint.AllowedTrustAnchors, proactiveResolver)
	}
	if endpoint := c.Endpoints.TrustMarkStatusEndpoint; endpoint.IsSet() {
		lh.AddTrustMarkStatusEndpoint(endpoint, backs.TrustMarks)
	}
	if endpoint := c.Endpoints.TrustMarkedEntitiesListingEndpoint; endpoint.IsSet() {
		lh.AddTrustMarkedEntitiesListingEndpoint(endpoint, backs.TrustMarks)
	}
	if endpoint := c.Endpoints.TrustMarkEndpoint; endpoint.IsSet() {
		lh.AddTrustMarkEndpoint(endpoint.EndpointConf, backs.TrustMarks, trustMarkCheckerMap)
	}
	if endpoint := c.Endpoints.TrustMarkRequestEndpoint; endpoint.IsSet() {
		lh.AddTrustMarkRequestEndpoint(endpoint, backs.TrustMarks)
	}
	if endpoint := c.Endpoints.HistoricalKeysEndpoint; endpoint.IsSet() {
		lh.AddHistoricalKeysEndpoint(
			endpoint, func() (jwx.JWKS, error) {
				kmsHistory, err := kmsManagedPKs.GetHistorical()
				if err != nil {
					return jwx.JWKS{}, err
				}
				apiHistory, err := apiManagedPKs.GetHistorical()
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
		)
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
			pec.Start()
			collector = pec
		}
		lh.AddEntityCollectionEndpoint(
			endpoint.EndpointConf, collector, endpoint.AllowedTrustAnchors, endpoint.PaginationLimit > 0,
		)
	}
	log.Info("Added Endpoints")

	lh.Start()
}
