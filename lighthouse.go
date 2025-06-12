package lighthouse

import (
	"crypto"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/lestrrat-go/jwx/v3/jwa"

	"github.com/go-oidfed/lib/pkg"
	"github.com/go-oidfed/lib/pkg/cache"
	"github.com/go-oidfed/lib/pkg/constants"
	"github.com/go-oidfed/lib/pkg/unixtime"

	"github.com/go-oidfed/lighthouse/internal"
	"github.com/go-oidfed/lighthouse/storage"
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
	*pkg.FederationEntity
	*pkg.TrustMarkIssuer
	*pkg.GeneralJWTSigner
	SubordinateStatementsConfig
	server *fiber.App
}

// SubordinateStatementsConfig is a type for setting MetadataPolicies and additional attributes that should go into the
// SubordinateStatements issued by this LightHouse
type SubordinateStatementsConfig struct {
	MetadataPolicies             *pkg.MetadataPolicies
	SubordinateStatementLifetime int64
	Constraints                  *pkg.ConstraintSpecification
	CriticalExtensions           []string
	MetadataPolicyCrit           []pkg.PolicyOperatorName
	Extra                        map[string]any
}

// NewLightHouse creates a new LightHouse
func NewLightHouse(
	entityID string, authorityHints []string, metadata *pkg.Metadata,
	privateSigningKey crypto.Signer, signingAlg jwa.SignatureAlgorithm, configurationLifetime int64,
	stmtConfig SubordinateStatementsConfig,
) (
	*LightHouse,
	error,
) {
	generalSigner := pkg.NewGeneralJWTSigner(privateSigningKey, signingAlg)
	fed, err := pkg.NewFederationEntity(
		entityID, authorityHints, metadata, generalSigner.EntityStatementSigner(), configurationLifetime,
	)
	if err != nil {
		return nil, err
	}
	if fed.Metadata == nil {
		fed.Metadata = &pkg.Metadata{}
	}
	if fed.Metadata.FederationEntity == nil {
		fed.Metadata.FederationEntity = &pkg.FederationEntityMetadata{}
	}
	server := fiber.New()
	server.Use(recover.New())
	server.Use(compress.New())
	server.Use(logger.New())
	entity := &LightHouse{
		FederationEntity:            fed,
		TrustMarkIssuer:             pkg.NewTrustMarkIssuer(entityID, generalSigner.TrustMarkSigner(), nil),
		GeneralJWTSigner:            generalSigner,
		SubordinateStatementsConfig: stmtConfig,
		server:                      server,
	}
	server.Get(
		"/.well-known/openid-federation", func(ctx *fiber.Ctx) error {
			cacheKey := cache.Key(cache.KeyEntityConfiguration, entityID)
			var cached []byte
			set, err := cache.Get(cacheKey, &cached)
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(pkg.ErrorServerError(err.Error()))
			}
			if set {
				ctx.Set(fiber.HeaderContentType, constants.ContentTypeEntityStatement)
				return ctx.Send(cached)
			}
			jwt, err := entity.EntityConfigurationJWT()
			if err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(pkg.ErrorServerError(err.Error()))
			}
			err = cache.Set(cacheKey, jwt, entityConfigurationCachePeriod)
			if err != nil {
				log.Println(err.Error())
			}
			ctx.Set(fiber.HeaderContentType, constants.ContentTypeEntityStatement)
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

// CreateSubordinateStatement returns a pkg.EntityStatementPayload for the passed storage.SubordinateInfo
func (fed LightHouse) CreateSubordinateStatement(subordinate *storage.SubordinateInfo) pkg.EntityStatementPayload {
	now := time.Now()
	return pkg.EntityStatementPayload{
		Issuer:             fed.FederationEntity.EntityID,
		Subject:            subordinate.EntityID,
		IssuedAt:           unixtime.Unixtime{Time: now},
		ExpiresAt:          unixtime.Unixtime{Time: now.Add(time.Duration(fed.SubordinateStatementLifetime) * time.Second)},
		SourceEndpoint:     fed.Metadata.FederationEntity.FederationFetchEndpoint,
		JWKS:               subordinate.JWKS,
		Metadata:           subordinate.Metadata,
		MetadataPolicy:     fed.MetadataPolicies,
		Constraints:        fed.Constraints,
		CriticalExtensions: fed.CriticalExtensions,
		MetadataPolicyCrit: fed.MetadataPolicyCrit,
		TrustMarks:         subordinate.TrustMarks,
		Extra:              internal.MergeMaps(true, fed.Extra, subordinate.Extra),
	}
}
