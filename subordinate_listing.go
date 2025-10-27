package lighthouse

import (
	"slices"

	arrays "github.com/adam-hanna/arrayOperations"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddSubordinateListingEndpoint adds a subordinate listing endpoint
func (fed *LightHouse) AddSubordinateListingEndpoint(
	endpoint EndpointConf, store model.SubordinateStorageBackend,
	trustMarkStore model.TrustMarkedEntitiesStorageBackend,
) {
	fed.fedMetadata.FederationListEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	if endpoint.Path == "" {
		return
	}
	fed.server.Get(
		endpoint.Path, func(ctx *fiber.Ctx) error {
			return handleSubordinateListing(
				ctx, ctx.Query("entity_type"), ctx.QueryBool("trust_marked"),
				ctx.Query("trust_mark_type"),
				ctx.QueryBool("intermediate"),
				store.Active(),
				trustMarkStore,
			)
		},
	)
}

func filterEntityType(info model.SubordinateInfo, value any) bool {
	v, ok := value.(string)
	return ok && slices.Contains(info.EntityTypes.ToStrings(), v)
}

func handleSubordinateListing(
	ctx *fiber.Ctx, entityType string, trustMarked bool, trustMarkType string,
	intermediate bool, q model.SubordinateStorageQuery,
	trustMarkedEntitiesStorage model.TrustMarkedEntitiesStorageBackend,
) error {
	if intermediate {
		ctx.Status(fiber.StatusBadRequest)
		return ctx.JSON(oidfed.ErrorUnsupportedParameter("parameter 'intermediate' is not supported"))
	}
	if trustMarkedEntitiesStorage == nil {
		if trustMarked {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorUnsupportedParameter("parameter 'trust_marked' is not supported"))
		}
		if trustMarkType != "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorUnsupportedParameter("parameter 'trust_mark_type' is not supported"))
		}
	}

	if q == nil {
		return ctx.JSON([]string{})
	}
	if entityType != "" {
		if err := q.AddFilter(filterEntityType, entityType); err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
	}

	ids, err := q.EntityIDs()
	if err != nil {
		ctx.Status(fiber.StatusInternalServerError)
		return ctx.JSON(oidfed.ErrorServerError(err.Error()))
	}

	if trustMarkType != "" || trustMarked {
		trustMarkedEntities, err := trustMarkedEntitiesStorage.Active(trustMarkType)
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		ids = arrays.Intersect(ids, trustMarkedEntities)
	}

	return ctx.JSON(ids)
}
