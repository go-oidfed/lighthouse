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
// All write operations are wrapped in transactions for atomicity.
func registerSubordinateKeys(
	r fiber.Router,
	storages model.Backends,
) {
	g := r.Group("/subordinates/:subordinateID/jwks")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - Get subordinate JWKS
	g.Get("/", handleGetSubordinateJWKS(storages.Subordinates))

	// PUT / - Replace subordinate JWKS (transactional)
	withCacheWipe.Put("/", handlePutSubordinateJWKS(storages))

	// POST / - Add JWK to subordinate JWKS (transactional)
	withCacheWipe.Post("/", handlePostSubordinateJWK(storages))

	// DELETE /:kid - Remove JWK by kid from subordinate JWKS (transactional)
	withCacheWipe.Delete("/:kid", handleDeleteSubordinateJWK(storages))
}

func handleGetSubordinateJWKS(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, ok := handleSubordinateLookup(c, subordinates)
		if !ok {
			return nil
		}
		// Return empty JWKS if none exists
		if info.JWKS.Keys.Set == nil {
			return c.JSON(fiber.Map{"keys": []any{}})
		}
		return c.JSON(info.JWKS)
	}
}

func handlePutSubordinateJWKS(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var body model.JWKS
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}

		var result *model.JWKS
		err := storages.InTransaction(func(tx *model.Backends) error {
			// Verify subordinate exists
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}

			updatedJWKS, err := tx.Subordinates.UpdateJWKSByDBID(id, body)
			if err != nil {
				return err
			}
			result = updatedJWKS
			// Record JWKS replaced event
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeJWKSReplaced)
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.JSON(result)
	}
}

func handlePostSubordinateJWK(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
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

		var result *model.JWKS
		err = storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}

			// Initialize JWKS if nil
			if info.JWKS.Keys.Set == nil {
				info.JWKS.Keys = jwx.NewJWKS()
			}
			// Add key to set
			if err := info.JWKS.Keys.AddKey(key); err != nil {
				return err
			}
			// Use UpdateJWKSByDBID to properly persist and get correct ID
			updatedJWKS, err := tx.Subordinates.UpdateJWKSByDBID(id, info.JWKS)
			if err != nil {
				return err
			}
			result = updatedJWKS
			// Record JWK added event
			kid, _ := key.KeyID()
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeJWKAdded, WithMessage("key added: "+kid))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	}
}

func handleDeleteSubordinateJWK(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		kid := c.Params("kid")

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}

			if info.JWKS.Keys.Set == nil {
				return nil // Nothing to delete
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
				return nil // Key not found, nothing to delete
			}
			// Persist the updated JWKS
			if _, err = tx.Subordinates.UpdateJWKSByDBID(id, info.JWKS); err != nil {
				return err
			}
			// Record JWK removed event
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeJWKRemoved, WithMessage("key removed: "+kid))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}
