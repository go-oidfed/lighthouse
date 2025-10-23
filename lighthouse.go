package lighthouse

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/lestrrat-go/jwx/v3/jwa"
	log "github.com/sirupsen/logrus"

	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/cache"
	"github.com/go-oidfed/lib/unixtime"

	"github.com/go-oidfed/lighthouse/internal/utils"
	"github.com/go-oidfed/lighthouse/internal/version"
	"github.com/go-oidfed/lighthouse/storage/model"
)

const entityConfigurationCachePeriod = 5 * time.Second

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
	*oidfed.FederationEntity
	*oidfed.TrustMarkIssuer
	*jwx.GeneralJWTSigner
	SubordinateStatementsConfig
	server     *fiber.App
	serverConf ServerConf
}

// SubordinateStatementsConfig is a type for setting MetadataPolicies and additional attributes that should go into the
// SubordinateStatements issued by this LightHouse
type SubordinateStatementsConfig struct {
	MetadataPolicies             *oidfed.MetadataPolicies
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
	entityID string, authorityHints []string, metadata *oidfed.Metadata,
	signer jwx.VersatileSigner, signingAlg jwa.SignatureAlgorithm,
	configurationLifetime time.Duration,
	stmtConfig SubordinateStatementsConfig, extra map[string]any,
) (
	*LightHouse,
	error,
) {
	if extra == nil {
		extra = make(map[string]any)
	}
	extra["lighthouse_version"] = version.VERSION
	generalSigner := jwx.NewGeneralJWTSigner(signer, []jwa.SignatureAlgorithm{signingAlg})
	fed, err := oidfed.NewFederationEntity(
		entityID, authorityHints, metadata, generalSigner.EntityStatementSigner(), configurationLifetime, extra,
	)
	if err != nil {
		return nil, err
	}
	if fed.Metadata == nil {
		fed.Metadata = &oidfed.Metadata{}
	}
	if fed.Metadata.FederationEntity == nil {
		fed.Metadata.FederationEntity = &oidfed.FederationEntityMetadata{}
	}
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
		FederationEntity:            fed,
		TrustMarkIssuer:             oidfed.NewTrustMarkIssuer(entityID, generalSigner.TrustMarkSigner(), nil),
		GeneralJWTSigner:            generalSigner,
		SubordinateStatementsConfig: stmtConfig,
		server:                      server,
		serverConf:                  serverConf,
	}
	server.Get(
		"/.well-known/openid-federation", func(ctx *fiber.Ctx) error {
			cacheKey := cache.Key(cache.KeyEntityConfiguration, entityID)
			var cached []byte
			set, err := cache.Get(cacheKey, &cached)
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			if set {
				ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeEntityStatement)
				return ctx.Send(cached)
			}
			jwt, err := entity.EntityConfigurationJWT()
			if err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			err = cache.Set(cacheKey, jwt, entityConfigurationCachePeriod)
			if err != nil {
				log.Println(err.Error())
			}
			ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeEntityStatement)
			return ctx.Send(jwt)
		},
	)
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

func (fed LightHouse) Start() {
	conf := fed.serverConf
	if !conf.TLS.Enabled {
		log.WithField("port", conf.Port).Info("TLS is disabled starting http server")
		log.WithError(fed.server.Listen(fmt.Sprintf(":%d", conf.Port))).Fatal()
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
			log.WithError(httpServer.Listen(":80")).Fatal()
		}()
	}
	time.Sleep(time.Millisecond) // This is just for a more pretty output with the tls header printed after the http one
	log.Info("TLS enabled, starting https server on port 443")
	log.WithError(fed.server.ListenTLS(":443", conf.TLS.Cert, conf.TLS.Key)).Fatal()
}

// CreateSubordinateStatement returns an oidfed.EntityStatementPayload for the passed storage.SubordinateInfo
func (fed LightHouse) CreateSubordinateStatement(subordinate *model.SubordinateInfo) oidfed.EntityStatementPayload {
	now := time.Now()
	return oidfed.EntityStatementPayload{
		Issuer:             fed.FederationEntity.EntityID,
		Subject:            subordinate.EntityID,
		IssuedAt:           unixtime.Unixtime{Time: now},
		ExpiresAt:          unixtime.Unixtime{Time: now.Add(fed.SubordinateStatementLifetime * time.Second)},
		SourceEndpoint:     fed.Metadata.FederationEntity.FederationFetchEndpoint,
		JWKS:               subordinate.JWKS.JWKS(),
		Metadata:           subordinate.Metadata,
		MetadataPolicy:     fed.MetadataPolicies,
		Constraints:        fed.Constraints,
		CriticalExtensions: fed.CriticalExtensions,
		MetadataPolicyCrit: fed.MetadataPolicyCrit,
		Extra:              utils.MergeMaps(true, fed.SubordinateStatementsConfig.Extra, map[string]any{}),
	}
}
