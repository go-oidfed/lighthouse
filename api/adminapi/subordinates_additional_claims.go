package adminapi

import (
	"errors"
	"strconv"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerSubordinateAdditionalClaims adds handlers for subordinate-specific additional claims.
func registerSubordinateAdditionalClaims(
	r fiber.Router,
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) {
	g := r.Group("/subordinates/:subordinateID/additional-claims")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - List all additional claims for a subordinate
	g.Get("/", handleListSubordinateAdditionalClaims(subordinates))

	// PUT / - Replace all additional claims for a subordinate
	withCacheWipe.Put("/", handlePutSubordinateAdditionalClaims(subordinates, recorder))

	// POST / - Create a single additional claim
	withCacheWipe.Post("/", handlePostSubordinateAdditionalClaim(subordinates, recorder))

	// GET /:additionalClaimsID - Get a single additional claim
	g.Get("/:additionalClaimsID", handleGetSubordinateAdditionalClaim(subordinates))

	// PUT /:additionalClaimsID - Update a single additional claim
	withCacheWipe.Put("/:additionalClaimsID", handleUpdateSubordinateAdditionalClaim(subordinates, recorder))

	// DELETE /:additionalClaimsID - Delete a single additional claim
	withCacheWipe.Delete("/:additionalClaimsID", handleDeleteSubordinateAdditionalClaim(subordinates, recorder))
}

func handleListSubordinateAdditionalClaims(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		claims, err := subordinates.ListAdditionalClaims(id)
		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			return writeServerError(c, err)
		}
		return c.JSON(claims)
	}
}

func handlePutSubordinateAdditionalClaims(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var req []model.AddAdditionalClaim
		if err := c.BodyParser(&req); err != nil {
			return writeBadBody(c)
		}
		claims, err := subordinates.SetAdditionalClaims(id, req)
		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			return writeServerError(c, err)
		}
		// Get subordinate ID for event recording
		info, _ := subordinates.GetByDBID(id)
		if info != nil {
			recorder.Record(info.ID, model.EventTypeClaimsUpdated)
		}
		return c.JSON(claims)
	}
}

func handlePostSubordinateAdditionalClaim(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var req model.AddAdditionalClaim
		if err := c.BodyParser(&req); err != nil {
			return writeBadBody(c)
		}
		claim, err := subordinates.CreateAdditionalClaim(id, req)
		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			var ae model.AlreadyExistsError
			if errors.As(err, &ae) {
				return writeConflict(c, err.Error())
			}
			return writeServerError(c, err)
		}
		// Get subordinate ID for event recording
		info, _ := subordinates.GetByDBID(id)
		if info != nil {
			recorder.Record(info.ID, model.EventTypeClaimsUpdated, WithMessage("claim: "+req.Claim))
		}
		return c.Status(fiber.StatusCreated).JSON(claim)
	}
}

func handleGetSubordinateAdditionalClaim(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		subID := c.Params("subordinateID")
		claimID := c.Params("additionalClaimsID")
		claim, err := subordinates.GetAdditionalClaim(subID, claimID)
		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			return writeServerError(c, err)
		}
		return c.JSON(claim)
	}
}

func handleUpdateSubordinateAdditionalClaim(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		subID := c.Params("subordinateID")
		claimID := c.Params("additionalClaimsID")
		var req model.AddAdditionalClaim
		if err := c.BodyParser(&req); err != nil {
			return writeBadBody(c)
		}
		claim, err := subordinates.UpdateAdditionalClaim(subID, claimID, req)
		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			var ae model.AlreadyExistsError
			if errors.As(err, &ae) {
				return writeConflict(c, err.Error())
			}
			return writeServerError(c, err)
		}
		// Get subordinate ID for event recording
		info, _ := subordinates.GetByDBID(subID)
		if info != nil {
			recorder.Record(info.ID, model.EventTypeClaimsUpdated, WithMessage("claim: "+req.Claim))
		}
		return c.JSON(claim)
	}
}

func handleDeleteSubordinateAdditionalClaim(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		subID := c.Params("subordinateID")
		claimID := c.Params("additionalClaimsID")
		// Get subordinate info for event recording before delete
		info, _ := subordinates.GetByDBID(subID)

		if err := subordinates.DeleteAdditionalClaim(subID, claimID); err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			return writeServerError(c, err)
		}
		if info != nil {
			recorder.Record(info.ID, model.EventTypeClaimDeleted, WithMessage("claim ID: "+claimID))
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// generalAdditionalClaim represents an additional claim stored in the KV store.
type generalAdditionalClaim struct {
	ID    int    `json:"id"`
	Claim string `json:"claim"`
	Value any    `json:"value"`
	Crit  bool   `json:"crit"`
}

// registerGeneralAdditionalClaims adds handlers for general additional claims applied to all subordinates.
func registerGeneralAdditionalClaims(r fiber.Router, kv model.KeyValueStore) {
	g := r.Group("/subordinates/additional-claims")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - List all general additional claims
	g.Get("/", handleListGeneralAdditionalClaims(kv))

	// PUT / - Replace all general additional claims
	withCacheWipe.Put("/", handlePutGeneralAdditionalClaims(kv))

	// POST / - Add a single general additional claim
	withCacheWipe.Post("/", handlePostGeneralAdditionalClaim(kv))

	// GET /:additionalClaimsID - Get a single general additional claim
	g.Get("/:additionalClaimsID", handleGetGeneralAdditionalClaim(kv))

	// PUT /:additionalClaimsID - Update a single general additional claim
	withCacheWipe.Put("/:additionalClaimsID", handleUpdateGeneralAdditionalClaim(kv))

	// DELETE /:additionalClaimsID - Delete a single general additional claim
	withCacheWipe.Delete("/:additionalClaimsID", handleDeleteGeneralAdditionalClaim(kv))
}

func loadGeneralAdditionalClaims(kv model.KeyValueStore) ([]generalAdditionalClaim, error) {
	var claims []generalAdditionalClaim
	found, err := kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, &claims)
	if err != nil {
		return nil, err
	}
	if !found {
		return []generalAdditionalClaim{}, nil
	}
	return claims, nil
}

func saveGeneralAdditionalClaims(kv model.KeyValueStore, claims []generalAdditionalClaim) error {
	return kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, claims)
}

func nextGeneralAdditionalClaimID(claims []generalAdditionalClaim) int {
	maxID := 0
	for _, c := range claims {
		if c.ID > maxID {
			maxID = c.ID
		}
	}
	return maxID + 1
}

func handleListGeneralAdditionalClaims(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := loadGeneralAdditionalClaims(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(claims)
	}
}

func handlePutGeneralAdditionalClaims(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req []model.AddAdditionalClaim
		if err := c.BodyParser(&req); err != nil {
			return writeBadBody(c)
		}
		claims := make([]generalAdditionalClaim, len(req))
		for i, r := range req {
			claims[i] = generalAdditionalClaim{
				ID:    i + 1,
				Claim: r.Claim,
				Value: r.Value,
				Crit:  r.Crit,
			}
		}
		if err := saveGeneralAdditionalClaims(kv, claims); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(claims)
	}
}

func handlePostGeneralAdditionalClaim(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req model.AddAdditionalClaim
		if err := c.BodyParser(&req); err != nil {
			return writeBadBody(c)
		}
		claims, err := loadGeneralAdditionalClaims(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		// Check for duplicate claim name
		for _, existing := range claims {
			if existing.Claim == req.Claim {
				return writeConflict(c, "claim already exists")
			}
		}
		newClaim := generalAdditionalClaim{
			ID:    nextGeneralAdditionalClaimID(claims),
			Claim: req.Claim,
			Value: req.Value,
			Crit:  req.Crit,
		}
		claims = append(claims, newClaim)
		if err = saveGeneralAdditionalClaims(kv, claims); err != nil {
			return writeServerError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(newClaim)
	}
}

func handleGetGeneralAdditionalClaim(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		idStr := c.Params("additionalClaimsID")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid additionalClaimsID"))
		}
		claims, err := loadGeneralAdditionalClaims(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		for _, claim := range claims {
			if claim.ID == id {
				return c.JSON(claim)
			}
		}
		return writeNotFound(c, "additional claim not found")
	}
}

func handleUpdateGeneralAdditionalClaim(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		idStr := c.Params("additionalClaimsID")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid additionalClaimsID"))
		}
		var req model.AddAdditionalClaim
		if err := c.BodyParser(&req); err != nil {
			return writeBadBody(c)
		}
		claims, err := loadGeneralAdditionalClaims(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		found := -1
		for i, claim := range claims {
			if claim.ID == id {
				found = i
				break
			}
		}
		if found == -1 {
			return writeNotFound(c, "additional claim not found")
		}
		// Check for duplicate claim name (excluding current)
		if req.Claim != "" && req.Claim != claims[found].Claim {
			for i, existing := range claims {
				if i != found && existing.Claim == req.Claim {
					return writeConflict(c, "claim already exists")
				}
			}
			claims[found].Claim = req.Claim
		}
		claims[found].Value = req.Value
		claims[found].Crit = req.Crit
		if err := saveGeneralAdditionalClaims(kv, claims); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(claims[found])
	}
}

func handleDeleteGeneralAdditionalClaim(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		idStr := c.Params("additionalClaimsID")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid additionalClaimsID"))
		}
		claims, err := loadGeneralAdditionalClaims(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		found := -1
		for i, claim := range claims {
			if claim.ID == id {
				found = i
				break
			}
		}
		if found == -1 {
			return writeNotFound(c, "additional claim not found")
		}
		claims = append(claims[:found], claims[found+1:]...)
		if err := saveGeneralAdditionalClaims(kv, claims); err != nil {
			return writeServerError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}
