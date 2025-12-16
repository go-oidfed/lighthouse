package adminapi

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	smodel "github.com/go-oidfed/lighthouse/storage/model"
)

func registerEntityConfiguration(
	r fiber.Router, addClaimsStore smodel.AdditionalClaimsStore, kv smodel.KeyValueStore,
	fedEntity oidfed.FederationEntity,
) {
	g := r.Group("/entity-configuration")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)
	g.Get(
		"/", func(c *fiber.Ctx) error {
			payload, err := fedEntity.EntityConfigurationPayload()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(payload)
		},
	)

	// Additional Claims collection (claim, value, crit)
	g.Get(
		"/additional-claims", func(c *fiber.Ctx) error {
			values, err := addClaimsStore.List()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(values)
		},
	)
	withCacheWipe.Put(
		"/additional-claims", func(c *fiber.Ctx) error {
			var req []smodel.AddAdditionalClaim
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			updated, err := addClaimsStore.Set(req)
			if err != nil {
				if isUniqueConstraintError(err) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("additional claim already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(updated)
		},
	)
	withCacheWipe.Post(
		"/additional-claims", func(c *fiber.Ctx) error {
			var req smodel.AddAdditionalClaim
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			row, err := addClaimsStore.Create(req)
			if err != nil {
				var alreadyExists smodel.AlreadyExistsError
				if errors.As(err, &alreadyExists) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("additional claim already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(row)
		},
	)
	g.Get(
		"/additional-claims/:additionalClaimsID", func(c *fiber.Ctx) error {
			idStr := c.Params("additionalClaimsID")
			id, err := strconv.ParseUint(idStr, 10, 64)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid additionalClaimsID"))
			}
			row, err := addClaimsStore.Get(strconv.FormatUint(id, 10))
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("additional claim not found"))
			}
			return c.JSON(row)
		},
	)
	withCacheWipe.Put(
		"/additional-claims/:additionalClaimsID", func(c *fiber.Ctx) error {
			id := c.Params("additionalClaimsID")
			var req smodel.AddAdditionalClaim
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			updated, err := addClaimsStore.Update(id, req)
			if err != nil {
				var notFound smodel.NotFoundError
				if errors.As(err, &notFound) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("additional claim not found"))
				}
				var alreadyExists smodel.AlreadyExistsError
				if errors.As(err, &alreadyExists) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("additional claim already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(updated)
		},
	)
	withCacheWipe.Delete(
		"/additional-claims/:additionalClaimsID",
		func(c *fiber.Ctx) error {
			idStr := c.Params("additionalClaimsID")
			id, err := strconv.ParseUint(idStr, 10, 64)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid additionalClaimsID"))
			}
			if err := addClaimsStore.Delete(strconv.FormatUint(id, 10)); err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("additional claim not found"))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	g.Get(
		"/lifetime", func(c *fiber.Ctx) error {
			var seconds int
			found, err := kv.GetAs(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyLifetime, &seconds)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found {
				seconds = 0
			}
			return c.JSON(seconds)
		},
	)
	withCacheWipe.Put(
		"/lifetime", func(c *fiber.Ctx) error {
			// Expect body to be a JSON integer
			if len(c.Body()) == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("empty body"))
			}
			var seconds int
			if err := json.Unmarshal(c.Body(), &seconds); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if seconds < 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("lifetime must be non-negative"))
			}
			if err := kv.SetAny(
				smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyLifetime, seconds,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(seconds)
		},
	)

	// Metadata
	g.Get(
		"/metadata", func(c *fiber.Ctx) error {
			rawAll, err := kv.Get(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if rawAll == nil {
				return c.JSON(fiber.Map{})
			}
			var meta oidfed.Metadata
			if err := json.Unmarshal(rawAll, &meta); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError("invalid stored metadata"))
			}
			return c.JSON(meta)
		},
	)
	withCacheWipe.Put(
		"/metadata", func(c *fiber.Ctx) error {
			var meta oidfed.Metadata
			if err := c.BodyParser(&meta); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			buf, _ := json.Marshal(meta)
			if err := kv.Set(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata, buf); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(meta)
		},
	)
	g.Get(
		"/metadata/:entityType/:claim", func(c *fiber.Ctx) error {
			entityType := c.Params("entityType")
			claim := c.Params("claim")
			rawAll, err := kv.Get(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if rawAll == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("metadata not found"))
			}
			var meta map[string]map[string]json.RawMessage
			if err := json.Unmarshal(rawAll, &meta); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError("invalid stored metadata"))
			}
			if m, ok := meta[entityType]; ok {
				if v, ok := m[claim]; ok {
					c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
					return c.Send(v)
				}
			}
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("metadata not found"))
		},
	)
	withCacheWipe.Put(
		"/metadata/:entityType/:claim", func(c *fiber.Ctx) error {
			entityType := c.Params("entityType")
			claim := c.Params("claim")
			if len(c.Body()) == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("empty body"))
			}
			var meta map[string]map[string]json.RawMessage
			if rawAll, err := kv.Get(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			} else if rawAll != nil {
				if err := json.Unmarshal(rawAll, &meta); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError("invalid stored metadata"))
				}
			}
			if meta == nil {
				meta = make(map[string]map[string]json.RawMessage)
			}
			if _, ok := meta[entityType]; !ok {
				meta[entityType] = make(map[string]json.RawMessage)
			}
			meta[entityType][claim] = json.RawMessage(c.Body())
			buf, _ := json.Marshal(meta)
			if err := kv.Set(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata, buf); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
			return c.Send(c.Body())
		},
	)
	withCacheWipe.Delete(
		"/metadata/:entityType/:claim", func(c *fiber.Ctx) error {
			entityType := c.Params("entityType")
			claim := c.Params("claim")
			var meta map[string]map[string]json.RawMessage
			rawAll, err := kv.Get(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if rawAll == nil {
				return c.SendStatus(fiber.StatusNoContent)
			}
			if err := json.Unmarshal(rawAll, &meta); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError("invalid stored metadata"))
			}
			if m, ok := meta[entityType]; ok {
				delete(m, claim)
				if len(m) == 0 {
					delete(meta, entityType)
				}
				buf, _ := json.Marshal(meta)
				if err := kv.Set(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata, buf); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
	g.Get(
		"/metadata/:entityType", func(c *fiber.Ctx) error {
			entityType := c.Params("entityType")
			var meta map[string]map[string]json.RawMessage
			if rawAll, err := kv.Get(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			} else if rawAll != nil {
				if err := json.Unmarshal(rawAll, &meta); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError("invalid stored metadata"))
				}
			}
			if meta == nil {
				meta = make(map[string]map[string]json.RawMessage)
			}
			claims := meta[entityType]
			if claims == nil {
				claims = map[string]json.RawMessage{}
			}
			return c.JSON(claims)
		},
	)
	withCacheWipe.Put(
		"/metadata/:entityType", func(c *fiber.Ctx) error {
			entityType := c.Params("entityType")
			var body map[string]json.RawMessage
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			var meta map[string]map[string]json.RawMessage
			if rawAll, err := kv.Get(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			} else if rawAll != nil {
				if err := json.Unmarshal(rawAll, &meta); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError("invalid stored metadata"))
				}
			}
			if meta == nil {
				meta = make(map[string]map[string]json.RawMessage)
			}
			meta[entityType] = body
			buf, _ := json.Marshal(meta)
			if err := kv.Set(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata, buf); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(body)
		},
	)
	withCacheWipe.Post(
		"/metadata/:entityType", func(c *fiber.Ctx) error {
			entityType := c.Params("entityType")
			var body map[string]json.RawMessage
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			var meta map[string]map[string]json.RawMessage
			if rawAll, err := kv.Get(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			} else if rawAll != nil {
				if err := json.Unmarshal(rawAll, &meta); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError("invalid stored metadata"))
				}
			}
			if meta == nil {
				meta = make(map[string]map[string]json.RawMessage)
			}
			if _, ok := meta[entityType]; !ok {
				meta[entityType] = make(map[string]json.RawMessage)
			}
			for claim, raw := range body {
				meta[entityType][claim] = raw
			}
			buf, _ := json.Marshal(meta)
			if err := kv.Set(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata, buf); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(body)
		},
	)
	withCacheWipe.Delete(
		"/metadata/:entityType", func(c *fiber.Ctx) error {
			entityType := c.Params("entityType")
			var meta map[string]map[string]json.RawMessage
			rawAll, err := kv.Get(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if rawAll == nil {
				return c.SendStatus(fiber.StatusNoContent)
			}
			if err := json.Unmarshal(rawAll, &meta); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError("invalid stored metadata"))
			}
			if meta != nil {
				delete(meta, entityType)
				buf, _ := json.Marshal(meta)
				if err := kv.Set(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata, buf); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

}

// isUniqueConstraintError performs a cheap check across supported drivers.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// sqlite | mysql | postgres common markers
	if
	// SQLite
	(strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "constraint failed")) ||
		// MySQL
		(strings.Contains(msg, "Duplicate entry") || strings.Contains(msg, "Error 1062")) ||
		// Postgres
		(strings.Contains(msg, "duplicate key value") || strings.Contains(msg, "violates unique constraint")) {
		return true
	}
	return false
}
