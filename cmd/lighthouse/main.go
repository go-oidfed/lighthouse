package main

import (
	"os"
	"strings"

	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/cache"
	"github.com/go-oidfed/lib/jwx"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/go-oidfed/lighthouse"
	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
	"github.com/go-oidfed/lighthouse/internal/logger"
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
	if redisAddr := c.Caching.RedisAddr; redisAddr != "" {
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
	err := initKey()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Loaded signing key")
	for _, tmc := range c.Federation.TrustMarks {
		if err = tmc.Verify(
			c.Federation.EntityID, c.Endpoints.TrustMarkEndpoint.ValidateURL(c.Federation.EntityID),
			jwx.NewTrustMarkSigner(keys.Federation()),
		); err != nil {
			log.Fatal(err)
		}
	}

	subordinateStorage, trustMarkedEntitiesStorage, err := config.LoadStorageBackends(c.Storage)
	if err != nil {
		log.Fatal(err)
	}

	lh, err := lighthouse.NewLightHouse(
		config.Get().Server,
		c.Federation.EntityID, c.Federation.AuthorityHints,
		&oidfed.Metadata{
			FederationEntity: &oidfed.FederationEntityMetadata{
				Extra:            c.Federation.Metadata.ExtraFederationEntityMetadata,
				DisplayName:      c.Federation.Metadata.DisplayName,
				Description:      c.Federation.Metadata.Description,
				Keywords:         c.Federation.Metadata.Keywords,
				Contacts:         c.Federation.Metadata.Contacts,
				LogoURI:          c.Federation.Metadata.LogoURI,
				PolicyURI:        c.Federation.Metadata.PolicyURI,
				InformationURI:   c.Federation.Metadata.InformationURI,
				OrganizationName: c.Federation.Metadata.OrganizationName,
				OrganizationURI:  c.Federation.Metadata.OrganizationURI,
			},
		},
		keys.Federation(), c.Signing.Algorithm,
		c.Federation.ConfigurationLifetime.Duration(),
		lighthouse.SubordinateStatementsConfig{
			MetadataPolicies:             nil,
			SubordinateStatementLifetime: c.Endpoints.FetchEndpoint.StatementLifetime.Duration(),
			// TODO read all of this from config or a storage backend
		}, c.Federation.ExtraEntityConfigurationData,
	)
	if err != nil {
		panic(err)
	}

	lh.MetadataPolicies = c.Federation.MetadataPolicy
	lh.Constraints = c.Federation.Constraints
	lh.CriticalExtensions = c.Federation.CriticalExtensions
	lh.MetadataPolicyCrit = c.Federation.MetadataPolicyCrit
	lh.TrustMarks = c.Federation.TrustMarks
	lh.TrustMarkIssuers = c.Federation.TrustMarkIssuers
	lh.TrustMarkOwners = c.Federation.TrustMarkOwners

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
		lh.AddFetchEndpoint(endpoint.EndpointConf, subordinateStorage)
	}
	if endpoint := c.Endpoints.ListEndpoint; endpoint.IsSet() {
		lh.AddSubordinateListingEndpoint(endpoint, subordinateStorage, trustMarkedEntitiesStorage)
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
		lh.AddTrustMarkStatusEndpoint(endpoint, trustMarkedEntitiesStorage)
	}
	if endpoint := c.Endpoints.TrustMarkedEntitiesListingEndpoint; endpoint.IsSet() {
		lh.AddTrustMarkedEntitiesListingEndpoint(endpoint, trustMarkedEntitiesStorage)
	}
	if endpoint := c.Endpoints.TrustMarkEndpoint; endpoint.IsSet() {
		lh.AddTrustMarkEndpoint(endpoint.EndpointConf, trustMarkedEntitiesStorage, trustMarkCheckerMap)
	}
	if endpoint := c.Endpoints.TrustMarkRequestEndpoint; endpoint.IsSet() {
		lh.AddTrustMarkRequestEndpoint(endpoint, trustMarkedEntitiesStorage)
	}
	if endpoint := c.Endpoints.HistoricalKeysEndpoint; endpoint.IsSet() {
		lh.AddHistoricalKeysEndpoint(
			endpoint, func() jwx.JWKS {
				return keys.History(jwx.KeyStorageTypeFederation)
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
		lh.AddEnrollEndpoint(endpoint.EndpointConf, subordinateStorage, checker)
	}
	if endpoint := c.Endpoints.EnrollmentRequestEndpoint; endpoint.IsSet() {
		lh.AddEnrollRequestEndpoint(endpoint, subordinateStorage)
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
