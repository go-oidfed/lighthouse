package adminapi

import (
	"encoding/json"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v3/jwk"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerSubordinateKeys registers JWKS endpoints for subordinates.
func registerSubordinateKeys(
	r fiber.Router,
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) {
	g := r.Group("/subordinates/:subordinateID/jwks")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - Get subordinate JWKS
	g.Get("/", handleGetSubordinateJWKS(subordinates))

	// PUT / - Replace subordinate JWKS
	withCacheWipe.Put("/", handlePutSubordinateJWKS(subordinates, recorder))

	// POST / - Add JWK to subordinate JWKS
	withCacheWipe.Post("/", handlePostSubordinateJWK(subordinates, recorder))

	// DELETE /:kid - Remove JWK by kid from subordinate JWKS
	withCacheWipe.Delete("/:kid", handleDeleteSubordinateJWK(subordinates, recorder))
}

func handleGetSubordinateJWKS(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		// Return empty JWKS if none exists
		if info.JWKS.Keys.Set == nil {
			return c.JSON(fiber.Map{"keys": []any{}})
		}
		return c.JSON(info.JWKS)
	}
}

func handlePutSubordinateJWKS(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		// Verify subordinate exists
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		var body model.JWKS
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		updatedJWKS, err := subordinates.UpdateJWKSByDBID(id, body)
		if err != nil {
			return writeServerError(c, err)
		}
		// Record JWKS replaced event
		recorder.Record(info.ID, model.EventTypeJWKSReplaced)
		return c.JSON(updatedJWKS)
	}
}

func handlePostSubordinateJWK(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		// Parse single JWK from body
		var jwkMap map[string]any
		if err := json.Unmarshal(c.Body(), &jwkMap); err != nil {
			return writeBadBody(c)
		}
		// Convert to jwk.Key
		keyData, err := json.Marshal(jwkMap)
		if err != nil {
			return writeBadBody(c)
		}
		key, err := jwk.ParseKey(keyData)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid JWK: " + err.Error()))
		}
		// Initialize JWKS if nil
		if info.JWKS.Keys.Set == nil {
			info.JWKS.Keys = jwx.NewJWKS()
		}
		// Add key to set
		if err := info.JWKS.Keys.AddKey(key); err != nil {
			return writeServerError(c, err)
		}
		// Use UpdateJWKSByDBID to properly persist and get correct ID
		updatedJWKS, err := subordinates.UpdateJWKSByDBID(id, info.JWKS)
		if err != nil {
			return writeServerError(c, err)
		}
		// Record JWK added event
		kid, _ := key.KeyID()
		recorder.Record(info.ID, model.EventTypeJWKAdded, WithMessage("key added: "+kid))
		return c.Status(fiber.StatusCreated).JSON(updatedJWKS)
	}
}

func handleDeleteSubordinateJWK(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		kid := c.Params("kid")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.JWKS.Keys.Set == nil {
			return c.SendStatus(fiber.StatusNoContent)
		}
		// Find and remove the key with matching kid
		found := false
		for i := 0; i < info.JWKS.Keys.Len(); i++ {
			key, ok := info.JWKS.Keys.Key(i)
			if !ok {
				continue
			}
			keyID, _ := key.KeyID()
			if keyID == kid {
				_ = info.JWKS.Keys.RemoveKey(key)
				found = true
				break
			}
		}
		if !found {
			return c.SendStatus(fiber.StatusNoContent)
		}
		// Persist the updated JWKS
		if _, err = subordinates.UpdateJWKSByDBID(id, info.JWKS); err != nil {
			return writeServerError(c, err)
		}
		// Record JWK removed event
		recorder.Record(info.ID, model.EventTypeJWKRemoved, WithMessage("key removed: "+kid))
		return c.SendStatus(fiber.StatusNoContent)
	}
}
