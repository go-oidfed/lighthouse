package lighthouse

import (
	"encoding/json"
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

	"github.com/go-oidfed/lighthouse/api/adminapi"
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
	oidfed.FederationEntity
	*oidfed.TrustMarkIssuer
	*jwx.GeneralJWTSigner
	SubordinateStatementsConfig
	server                            *fiber.App
	serverConf                        ServerConf
	fedMetadata                       oidfed.FederationEntityMetadata
	entityConfigurationCritExtensions []string
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

func getMetadataFromDB(kvStorage model.KeyValueStore) (*oidfed.Metadata, error) {
	if kvStorage == nil {
		return nil, nil
	}
	raw, err := kvStorage.Get(
		model.KeyValueScopeEntityConfiguration,
		model.KeyValueKeyMetadata,
	)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var m oidfed.Metadata
	if err = json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
func getAuthorityHintsFromDB(store model.AuthorityHintsStore) ([]string, error) {
	if store == nil {
		return nil, nil
	}
	rows, err := store.List()
	if err != nil {
		return nil, err
	}
	hints := make([]string, 0, len(rows))
	for _, r := range rows {
		hints = append(hints, r.EntityID)
	}
	return hints, nil
}
func getEntityConfigurationLifetimeFromDB(kvStorage model.KeyValueStore) (time.Duration, error) {
	if kvStorage == nil {
		return 0, nil
	}
	var seconds int
	found, err := kvStorage.GetAs(model.KeyValueScopeEntityConfiguration, model.KeyValueKeyLifetime, &seconds)
	if err != nil {
		return 0, err
	}
	if !found || seconds <= 0 {
		return 24 * time.Hour, nil
	}
	return time.Duration(seconds) * time.Second, nil
}

func getEntityConfigurationAdditionalClaimsFromDB(store model.AdditionalClaimsStore) (
	map[string]any, []string,
	error,
) {
	extra := make(map[string]any)
	// Load additional claims for entity configuration as Extra
	if store == nil {
		return nil, nil, nil
	}
	rows, err := store.List()
	if err != nil {
		return nil, nil, err
	}
	var crits []string
	for _, row := range rows {
		extra[row.Claim] = row.Value
		if row.Crit {
			crits = append(crits, row.Claim)
		}
	}
	return extra, crits, nil
}

// NewLightHouse creates a new LightHouse
func NewLightHouse(
	serverConf ServerConf,
	entityID string,
	signer jwx.VersatileSigner, signingAlg jwa.SignatureAlgorithm,
	stmtConfig SubordinateStatementsConfig,
	storages model.Backends,
) (
	*LightHouse,
	error,
) {
	generalSigner := jwx.NewGeneralJWTSigner(signer, []jwa.SignatureAlgorithm{signingAlg})
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
	}

	entity.FederationEntity = &oidfed.DynamicFederationEntity{
		ID: entityID,
		Metadata: func() (*oidfed.Metadata, error) {
			m, err := getMetadataFromDB(storages.KV)
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
			return getAuthorityHintsFromDB(storages.AuthorityHints)
		},
		ConfigurationLifetime: func() (time.Duration, error) {
			return getEntityConfigurationLifetimeFromDB(storages.KV)
		},
		EntityStatementSigner: func() (*jwx.EntityStatementSigner, error) {
			return generalSigner.EntityStatementSigner(), nil
		},
		TrustMarks:       nil, //TODO
		TrustMarkIssuers: nil, //TODO
		TrustMarkOwners:  nil, //TODO
		Extra: func() (map[string]any, []string, error) {
			extra, crits, err := getEntityConfigurationAdditionalClaimsFromDB(storages.AdditionalClaims)
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
	if err := adminapi.Register(
		server.Group("/api/v1/admin"), entityID, storages, entity.FederationEntity,
	); err != nil {
		return nil, err
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
		Issuer:             fed.FederationEntity.EntityID(),
		Subject:            subordinate.EntityID,
		IssuedAt:           unixtime.Unixtime{Time: now},
		ExpiresAt:          unixtime.Unixtime{Time: now.Add(fed.SubordinateStatementLifetime * time.Second)},
		SourceEndpoint:     fed.fedMetadata.FederationFetchEndpoint,
		JWKS:               subordinate.JWKS.JWKS(),
		Metadata:           subordinate.Metadata,
		MetadataPolicy:     fed.MetadataPolicies,
		Constraints:        fed.Constraints,
		CriticalExtensions: fed.CriticalExtensions,
		MetadataPolicyCrit: fed.MetadataPolicyCrit,
		Extra:              utils.MergeMaps(true, fed.SubordinateStatementsConfig.Extra, map[string]any{}),
	}
}
