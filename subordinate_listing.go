package lighthouse

import (
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
			return handleSubordinateListing(ctx, store, trustMarkStore)
		},
	)
}

type SubordinateListingRequest struct {
	EntityType    []string `json:"entity_type" query:"entity_type"`
	Intermediate  bool     `json:"intermediate" query:"intermediate"`
	TrustMarked   bool     `json:"trust_marked" query:"trust_marked"`
	TrustMarkType string   `json:"trust_mark_type" query:"trust_mark_type"`
}

func handleSubordinateListing(
	ctx *fiber.Ctx, subordinates model.SubordinateStorageBackend,
	trustMarkedEntitiesStorage model.TrustMarkedEntitiesStorageBackend,
) error {
	var req SubordinateListingRequest
	if err := ctx.QueryParser(&req); err != nil {
		ctx.Status(fiber.StatusBadRequest)
		return ctx.JSON(oidfed.ErrorInvalidRequest("could not parse request parameters: " + err.Error()))
	}
	if ctx.Query("intermediate") != "" {
		ctx.Status(fiber.StatusBadRequest)
		return ctx.JSON(oidfed.ErrorUnsupportedParameter("parameter 'intermediate' is not supported"))
	}
	if trustMarkedEntitiesStorage == nil {
		if req.TrustMarked {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorUnsupportedParameter("parameter 'trust_marked' is not supported"))
		}
		if req.TrustMarkType != "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorUnsupportedParameter("parameter 'trust_mark_type' is not supported"))
		}
	}
	var infos []model.BasicSubordinateInfo
	var err error
	if req.EntityType != nil {
		infos, err = subordinates.GetByStatusAndAnyEntityType(model.StatusActive, req.EntityType)
	} else {
		infos, err = subordinates.GetByStatus(model.StatusActive)
	}
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	if len(infos) == 0 {
		return ctx.JSON([]string{})
	}

	ids := make([]string, len(infos))
	for i, info := range infos {
		ids[i] = info.EntityID
	}

	if req.TrustMarkType != "" || req.TrustMarked {
		trustMarkedEntities, err := trustMarkedEntitiesStorage.Active(req.TrustMarkType)
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		ids = arrays.Intersect(ids, trustMarkedEntities)
	}

	return ctx.JSON(ids)
}
