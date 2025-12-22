package lighthouse

import (
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
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	log "github.com/sirupsen/logrus"

	"github.com/go-oidfed/lighthouse/api/adminapi"
	"github.com/go-oidfed/lighthouse/internal"
	"github.com/go-oidfed/lighthouse/internal/utils"
	"github.com/go-oidfed/lighthouse/internal/version"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

const MaximumEntityConfigurationCachePeriod = 8 * time.Hour

// EndpointConf is a type for configuring an endpoint with an internal and external path
type EndpointConf struct {
	Path string `yaml:"path"`
	URL  string `yaml:"url"`
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
	SubordinateStatementsConfig
	server         *fiber.App
	adminAPIServer *fiber.App
	serverConf     ServerConf
	fedMetadata    oidfed.FederationEntityMetadata
	keyManagement  adminapi.KeyManagement
	LogoBanner     bool
	VersionBanner  bool
	storages       model.Backends
}

// SubordinateStatementsConfig is a type for setting MetadataPolicies and additional attributes that should go into the
// SubordinateStatements issued by this LightHouse
type SubordinateStatementsConfig struct {
	SubordinateStatementLifetime time.Duration
	Constraints                  *oidfed.ConstraintSpecification
	CriticalExtensions           []string
	MetadataPolicyCrit           []oidfed.PolicyOperatorName
	Extra                        map[string]any
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
	stmtConfig SubordinateStatementsConfig,
	storages model.Backends,
	admin AdminAPIOptions,
) (
	*LightHouse,
	error,
) {

	keyManagement, err := initKey(signingConf, storages)
	if err != nil {
		return nil, err
	}
	versatileSigner := kms.KMSToVersatileSignerWithJWKSFunc(
		keyManagement.BasicKeys, func() (jwx.JWKS, error) {
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
	)

	generalSigner := jwx.NewGeneralJWTSigner(versatileSigner, keyManagement.BasicKeys.GetAlgs())
	if tps := serverConf.TrustedProxies; len(tps) > 0 {
		FiberServerConfig.TrustedProxies = serverConf.TrustedProxies
		FiberServerConfig.EnableTrustedProxyCheck = true
	}
	FiberServerConfig.ProxyHeader = serverConf.ForwardedIPHeader
	server := fiber.New(FiberServerConfig)
	server.Use(recover.New())
	server.Use(compress.New())
	server.Use(logger.New())
	server.Use(requestid.New())
	entity := &LightHouse{
		TrustMarkIssuer:             oidfed.NewTrustMarkIssuer(entityID, generalSigner.TrustMarkSigner(), nil),
		GeneralJWTSigner:            generalSigner,
		SubordinateStatementsConfig: stmtConfig,
		server:                      server,
		serverConf:                  serverConf,
		LogoBanner:                  true,
		VersionBanner:               true,
		keyManagement:               keyManagement,
		storages:                    storages,
	}

	entity.FederationEntity = &oidfed.DynamicFederationEntity{
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
			return generalSigner.EntityStatementSigner(), nil
		},
		TrustMarks: nil, //TODO
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

	server.Get(
		"/.well-known/openid-federation", func(ctx *fiber.Ctx) error {
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
			err = cache.Set(
				internal.CacheKeyEntityConfiguration, jwt,
				min(MaximumEntityConfigurationCachePeriod, time.Until(ec.ExpiresAt.Time.Add(-1*time.Minute))),
			)
			if err != nil {
				log.WithError(err).Error("failed to cache entity configuration")
			}
			ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeEntityStatement)
			return ctx.Send(jwt)
		},
	)
	// Initialize Admin API according to options
	if admin.Enabled {
		if admin.Port > 0 && admin.Port != serverConf.Port {
			// Separate admin server
			adminApp := fiber.New(FiberServerConfig)
			adminApp.Use(recover.New())
			adminApp.Use(compress.New())
			adminApp.Use(logger.New())
			adminApp.Use(requestid.New())
			entity.adminAPIServer = adminApp
			entity.serverConf.AdminAPIPort = admin.Port
		} else {
			// Mount on main server
			entity.adminAPIServer = server
		}
		if err = adminapi.Register(
			entity.adminAPIServer.Group("/api/v1/admin"), entityID, storages,
			entity.FederationEntity,
			keyManagement,
			&adminapi.Options{
				UsersEnabled: admin.UsersEnabled,
				Port:         admin.Port,
			},
		); err != nil {
			return nil, err
		}
	}
	return entity, nil
}

// HttpHandlerFunc returns an http.HandlerFunc for serving all the necessary endpoints
func (fed LightHouse) HttpHandlerFunc() http.HandlerFunc {
	return adaptor.FiberApp(fed.server)
}

// Listen starts an http server at the specific address for serving all the
// necessary endpoints
func (fed LightHouse) Listen(addr string) error {
	return fed.server.Listen(addr)
}

//go:embed banner.txt
var bannerTxt string

func (fed LightHouse) banner() {
	bannerWidth := 0
	if fed.LogoBanner {
		bannerWidth = 104
		fmt.Println(bannerTxt)
	}
	if fed.VersionBanner {
		fmt.Println(version.Banner(bannerWidth))
	} else {
		log.WithField("version", version.VERSION).Info("Starting lighthouse")
	}
}

func (fed LightHouse) Start() {
	fed.banner()

	conf := fed.serverConf
	if fed.adminAPIServer != nil && fed.adminAPIServer != fed.server {
		log.WithField("port", conf.AdminAPIPort).Info("starting admin api server")
		go func() {
			log.WithError(fed.adminAPIServer.Listen(fmt.Sprintf("%s:%d", conf.IPListen, conf.AdminAPIPort))).Fatal()
		}()
	}
	if !conf.TLS.Enabled {
		log.WithField("port", conf.Port).Info("TLS is disabled starting http server")
		log.WithError(fed.server.Listen(fmt.Sprintf("%s:%d", conf.IPListen, conf.Port))).Fatal()
	}
	// TLS enabled
	if conf.TLS.RedirectHTTP {
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
		log.Info("TLS and http redirect enabled, starting redirect server on port 80")
		go func() {
			log.WithError(httpServer.Listen(conf.IPListen + ":80")).Fatal()
		}()
	}
	time.Sleep(time.Millisecond) // This is just for a more pretty output with the tls header printed after the http one
	log.Info("TLS enabled, starting https server on port 443")
	log.WithError(fed.server.ListenTLS(conf.IPListen+":443", conf.TLS.Cert, conf.TLS.Key)).Fatal()
}

// CreateSubordinateStatement returns an oidfed.EntityStatementPayload for the passed storage.ExtendedSubordinateInfo
func (fed LightHouse) CreateSubordinateStatement(subordinate *model.ExtendedSubordinateInfo) oidfed.EntityStatementPayload {
	now := time.Now()
	return oidfed.EntityStatementPayload{
		Issuer:         fed.FederationEntity.EntityID(),
		Subject:        subordinate.EntityID,
		IssuedAt:       unixtime.Unixtime{Time: now},
		ExpiresAt:      unixtime.Unixtime{Time: now.Add(fed.SubordinateStatementLifetime)},
		SourceEndpoint: fed.fedMetadata.FederationFetchEndpoint,
		JWKS:           subordinate.JWKS.Keys,
		Metadata:       subordinate.Metadata,
		MetadataPolicy: subordinate.MetadataPolicy,
		Constraints:    subordinate.Constraints,
		// CriticalExtensions: subordinate.Crit, //TODO
		MetadataPolicyCrit: subordinate.MetadataPolicyCrit.ToPolicyOperatorNames(),
		Extra:              utils.MergeMaps(true, fed.SubordinateStatementsConfig.Extra, map[string]any{}),
	}
}

// AdminAPIOptions controls initialization of the admin API.
type AdminAPIOptions struct {
	Enabled      bool
	UsersEnabled bool
	// Port: 0 mounts on main server under /api/v1/admin; >0 starts a separate server on this port
	Port int
}
