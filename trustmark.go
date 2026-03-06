package lighthouse

import (
	"time"

	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TrustMarkEndpointConfig holds configuration for the trust mark endpoint
type TrustMarkEndpointConfig struct {
	// Store for subject status (backward compatibility)
	Store model.TrustMarkedEntitiesStorageBackend
	// SpecStore for loading TrustMarkSpec from DB (new)
	SpecStore model.TrustMarkSpecStore
	// InstanceStore for tracking issued trust mark instances
	InstanceStore model.IssuedTrustMarkInstanceStore
	// Checkers map for backward compatibility (config-based checkers)
	Checkers map[string]EntityChecker
	// Cache for eligibility results
	Cache *EligibilityCache
}

// AddTrustMarkEndpoint adds a trust mark endpoint
func (fed *LightHouse) AddTrustMarkEndpoint(
	endpoint EndpointConf,
	store model.TrustMarkedEntitiesStorageBackend,
	checkers map[string]EntityChecker,
) {
	// Use the new implementation with a config struct
	fed.AddTrustMarkEndpointWithConfig(
		endpoint, TrustMarkEndpointConfig{
			Store:    store,
			Checkers: checkers,
		},
	)
}

// AddTrustMarkEndpointWithConfig adds a trust mark endpoint with full configuration
func (fed *LightHouse) AddTrustMarkEndpointWithConfig(
	endpoint EndpointConf,
	config TrustMarkEndpointConfig,
) {
	trustMarkEndpointURL := endpoint.ValidateURL(fed.FederationEntity.EntityID())
	fed.fedMetadata.FederationTrustMarkEndpoint = trustMarkEndpointURL
	// Update the trust mark config provider with the endpoint URL
	if fed.trustMarkConfigProvider != nil {
		fed.trustMarkConfigProvider.SetTrustMarkEndpoint(trustMarkEndpointURL)
	}
	if endpoint.Path == "" {
		return
	}
	fed.server.Get(
		endpoint.Path, func(ctx *fiber.Ctx) error {
			return fed.handleTrustMarkRequest(ctx, config)
		},
	)
}

// handleTrustMarkRequest handles a trust mark request with the new eligibility system
func (fed *LightHouse) handleTrustMarkRequest(
	ctx *fiber.Ctx,
	config TrustMarkEndpointConfig,
) error {
	trustMarkType := ctx.Query("trust_mark_type")
	sub := ctx.Query("sub")

	// Validate required parameters
	if sub == "" {
		ctx.Status(fiber.StatusBadRequest)
		return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'sub' not given"))
	}
	if trustMarkType == "" {
		ctx.Status(fiber.StatusBadRequest)
		return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'trust_mark_type' not given"))
	}

	// Check if we support this trust mark type (uses provider if configured)
	if !fed.TrustMarkIssuer.HasTrustMarkType(trustMarkType) {
		ctx.Status(fiber.StatusNotFound)
		return ctx.JSON(oidfed.ErrorNotFound("'trust_mark_type' not known"))
	}

	// Try to load TrustMarkSpec from DB for eligibility config
	var dbSpec *model.TrustMarkSpec
	var eligibilityConfig *model.EligibilityConfig
	if config.SpecStore != nil {
		spec, err := config.SpecStore.GetByType(trustMarkType)
		if err == nil {
			dbSpec = spec
			eligibilityConfig = spec.EligibilityConfig
		}
		// If not found in DB, continue with legacy behavior
	}

	// Default to db_only mode if no eligibility config
	if eligibilityConfig == nil {
		eligibilityConfig = &model.EligibilityConfig{Mode: model.EligibilityModeDBOnly}
	}

	// Check cache first (if enabled)
	if config.Cache != nil && eligibilityConfig.CheckCacheTTL > 0 {
		if eligible, httpCode, reason, found := config.Cache.Get(trustMarkType, sub); found {
			if eligible {
				return fed.issueAndSendTrustMarkWithClaims(ctx, trustMarkType, sub, dbSpec, config)
			}
			ctx.Status(httpCode)
			return ctx.JSON(
				&oidfed.Error{
					Error:            "not_eligible",
					ErrorDescription: reason,
				},
			)
		}
	}

	// Run eligibility check based on mode
	eligible, httpCode, reason := fed.checkEligibility(trustMarkType, sub, eligibilityConfig, config)

	// Cache result if caching is enabled
	if config.Cache != nil && eligibilityConfig.CheckCacheTTL > 0 {
		config.Cache.Set(
			trustMarkType, sub, eligible, httpCode, reason,
			time.Duration(eligibilityConfig.CheckCacheTTL)*time.Second,
		)
	}

	if eligible {
		return fed.issueAndSendTrustMarkWithClaims(ctx, trustMarkType, sub, dbSpec, config)
	}

	ctx.Status(httpCode)
	return ctx.JSON(
		&oidfed.Error{
			Error:            "not_eligible",
			ErrorDescription: reason,
		},
	)
}

// checkEligibility checks if a subject is eligible for a trust mark based on the eligibility mode
func (fed *LightHouse) checkEligibility(
	trustMarkType, sub string,
	eligibilityConfig *model.EligibilityConfig,
	config TrustMarkEndpointConfig,
) (eligible bool, httpCode int, reason string) {
	switch eligibilityConfig.Mode {
	case model.EligibilityModeDBOnly:
		return fed.checkDBEligibility(trustMarkType, sub, config)

	case model.EligibilityModeCheckOnly:
		return fed.runChecker(trustMarkType, sub, eligibilityConfig.Checker, config)

	case model.EligibilityModeDBOrCheck:
		// DB first, then checker
		if ok, _, _ := fed.checkDBEligibility(trustMarkType, sub, config); ok {
			return true, 0, ""
		}
		return fed.runChecker(trustMarkType, sub, eligibilityConfig.Checker, config)

	case model.EligibilityModeDBAndCheck:
		// Must pass both
		if ok, code, reason := fed.checkDBEligibility(trustMarkType, sub, config); !ok {
			return false, code, reason
		}
		return fed.runChecker(trustMarkType, sub, eligibilityConfig.Checker, config)

	case model.EligibilityModeCustom:
		return fed.runChecker(trustMarkType, sub, eligibilityConfig.Checker, config)

	default:
		return false, fiber.StatusInternalServerError, "unknown eligibility mode"
	}
}

// checkDBEligibility checks if a subject is eligible based on the database status
func (*LightHouse) checkDBEligibility(
	trustMarkType, sub string,
	config TrustMarkEndpointConfig,
) (eligible bool, httpCode int, reason string) {
	if config.Store == nil {
		return false, fiber.StatusInternalServerError, "no store configured"
	}

	status, err := config.Store.TrustMarkedStatus(trustMarkType, sub)
	if err != nil {
		return false, fiber.StatusInternalServerError, err.Error()
	}

	switch status {
	case model.StatusActive:
		return true, 0, ""
	case model.StatusBlocked:
		return false, fiber.StatusForbidden, "subject is blocked from this trust mark"
	case model.StatusPending:
		return false, fiber.StatusAccepted, "approval pending"
	default: // StatusInactive or unknown
		return false, fiber.StatusNotFound, "subject not in active list for this trust mark type"
	}
}

// runChecker runs an entity checker against a subject
func (*LightHouse) runChecker(
	trustMarkType, sub string,
	checkerConfig *model.CheckerConfig,
	config TrustMarkEndpointConfig,
) (eligible bool, httpCode int, reason string) {
	var checker EntityChecker

	// Try to build checker from config
	if checkerConfig != nil {
		var err error
		checker, err = EntityCheckerFromJSONConfig(checkerConfig.Type, checkerConfig.Config)
		if err != nil {
			return false, fiber.StatusInternalServerError, "failed to build checker: " + err.Error()
		}

		// If it's a contextual checker (like db_list), set the context
		if contextual, ok := checker.(ContextualEntityChecker); ok {
			contextual.SetContext(
				CheckerContext{
					Store:         config.Store,
					TrustMarkType: trustMarkType,
				},
			)
		}
	}

	// Fall back to config-provided checkers (backward compatibility)
	if checker == nil && config.Checkers != nil {
		checker = config.Checkers[trustMarkType]
	}

	// No checker means not eligible
	if checker == nil {
		return false, fiber.StatusNotFound, "no checker configured and subject not in database"
	}

	// Get entity configuration
	entityConfig, err := oidfed.GetEntityConfiguration(sub)
	if err != nil {
		return false, fiber.StatusBadRequest, "could not obtain entity configuration: " + err.Error()
	}

	// Run the checker
	ok, code, errResponse := checker.Check(entityConfig, entityConfig.Metadata.GuessEntityTypes())
	if !ok {
		httpCode := fiber.StatusForbidden
		if code != 0 {
			httpCode = code
		}
		msg := "entity check failed"
		if errResponse != nil {
			msg = errResponse.ErrorDescription
		}
		return false, httpCode, msg
	}

	return true, 0, ""
}

// issueAndSendTrustMarkWithClaims issues a trust mark with merged additional claims
func (fed *LightHouse) issueAndSendTrustMarkWithClaims(
	ctx *fiber.Ctx,
	trustMarkType, sub string,
	dbSpec *model.TrustMarkSpec,
	config TrustMarkEndpointConfig,
) error {
	// Get subject-specific additional claims
	var subjectClaims map[string]any
	if dbSpec != nil && config.SpecStore != nil {
		// Try to get subject-specific claims
		subject, err := config.SpecStore.GetSubject(dbSpec.TrustMarkType, sub)
		if err == nil && subject != nil {
			subjectClaims = subject.AdditionalClaims
		}
	}

	// Generate JTI (JWT ID) for this issuance
	jti := uuid.New().String()

	// Merge JTI into subject claims
	if subjectClaims == nil {
		subjectClaims = make(map[string]any)
	}
	subjectClaims["jti"] = jti

	// Use IssueTrustMarkWithOptions which handles claim merging
	// (spec.Extra claims are already loaded via the TrustMarkSpecProvider)
	tm, expiresAt, err := fed.IssueTrustMarkWithOptions(
		trustMarkType, sub, oidfed.IssueTrustMarkOptions{
			SubjectClaims: subjectClaims,
		},
	)
	if err != nil {
		ctx.Status(fiber.StatusInternalServerError)
		return ctx.JSON(oidfed.ErrorServerError(err.Error()))
	}

	// Persist the issued instance for status tracking and revocation
	if config.InstanceStore != nil {
		instance := &model.IssuedTrustMarkInstance{
			JTI:           jti,
			TrustMarkType: trustMarkType,
			Subject:       sub,
			Revoked:       false,
		}

		// Set expiration if available
		if expiresAt != nil {
			instance.ExpiresAt = int(expiresAt.Time.Unix())
		}

		// Try to link to TrustMarkSubject record if it exists
		if subjectID, err := config.InstanceStore.FindSubjectID(trustMarkType, sub); err == nil && subjectID > 0 {
			instance.TrustMarkSubjectID = subjectID
		}

		if err := config.InstanceStore.Create(instance); err != nil {
			// Log the error but don't fail the request - the trust mark was issued successfully
			log.WithError(err).WithFields(log.Fields{
				"jti":             jti,
				"trust_mark_type": trustMarkType,
				"subject":         sub,
			}).Warn("failed to persist issued trust mark instance")
		}
	}

	ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeTrustMark)
	return ctx.SendString(tm)
}
