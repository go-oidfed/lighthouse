package adminapi

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage"
	smodel "github.com/go-oidfed/lighthouse/storage/model"
)

// metadataStore wraps KV operations for metadata to reduce duplication.
type metadataStore struct {
	kv smodel.KeyValueStore
}

func (m *metadataStore) load() (map[string]map[string]json.RawMessage, error) {
	rawAll, err := m.kv.Get(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata)
	if err != nil {
		return nil, err
	}
	if rawAll == nil {
		return make(map[string]map[string]json.RawMessage), nil
	}
	var meta map[string]map[string]json.RawMessage
	if err := json.Unmarshal(rawAll, &meta); err != nil {
		return nil, errors.New("invalid stored metadata")
	}
	return meta, nil
}

func (m *metadataStore) save(meta map[string]map[string]json.RawMessage) error {
	buf, _ := json.Marshal(meta)
	return m.kv.Set(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata, buf)
}

// Error response helpers
func serverError(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(msg))
}

func badRequest(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(msg))
}

func notFound(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(msg))
}

func conflict(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(msg))
}

// entityConfigHandlers groups handlers for entity configuration endpoints.
type entityConfigHandlers struct {
	fedEntity oidfed.FederationEntity
}

func (h *entityConfigHandlers) get(c *fiber.Ctx) error {
	payload, err := h.fedEntity.EntityConfigurationPayload()
	if err != nil {
		return serverError(c, err.Error())
	}
	return c.JSON(payload)
}

// additionalClaimsHandlers groups handlers for additional claims endpoints.
type additionalClaimsHandlers struct {
	store smodel.AdditionalClaimsStore
}

func (h *additionalClaimsHandlers) list(c *fiber.Ctx) error {
	values, err := h.store.List()
	if err != nil {
		return serverError(c, err.Error())
	}
	return c.JSON(values)
}

func (h *additionalClaimsHandlers) set(c *fiber.Ctx) error {
	var req []smodel.AddAdditionalClaim
	if err := c.BodyParser(&req); err != nil {
		return badRequest(c, "invalid body")
	}
	updated, err := h.store.Set(req)
	if err != nil {
		if isUniqueConstraintError(err) {
			return conflict(c, "additional claim already exists")
		}
		return serverError(c, err.Error())
	}
	return c.JSON(updated)
}

func (h *additionalClaimsHandlers) create(c *fiber.Ctx) error {
	var req smodel.AddAdditionalClaim
	if err := c.BodyParser(&req); err != nil {
		return badRequest(c, "invalid body")
	}
	row, err := h.store.Create(req)
	if err != nil {
		var alreadyExists smodel.AlreadyExistsError
		if errors.As(err, &alreadyExists) {
			return conflict(c, "additional claim already exists")
		}
		return serverError(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(row)
}

func (h *additionalClaimsHandlers) get(c *fiber.Ctx) error {
	idStr := c.Params("additionalClaimsID")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return badRequest(c, "invalid additionalClaimsID")
	}
	row, err := h.store.Get(strconv.FormatUint(id, 10))
	if err != nil {
		return notFound(c, "additional claim not found")
	}
	return c.JSON(row)
}

func (h *additionalClaimsHandlers) update(c *fiber.Ctx) error {
	id := c.Params("additionalClaimsID")
	var req smodel.AddAdditionalClaim
	if err := c.BodyParser(&req); err != nil {
		return badRequest(c, "invalid body")
	}
	updated, err := h.store.Update(id, req)
	if err != nil {
		var notFoundErr smodel.NotFoundError
		if errors.As(err, &notFoundErr) {
			return notFound(c, "additional claim not found")
		}
		var alreadyExists smodel.AlreadyExistsError
		if errors.As(err, &alreadyExists) {
			return conflict(c, "additional claim already exists")
		}
		return serverError(c, err.Error())
	}
	return c.JSON(updated)
}

func (h *additionalClaimsHandlers) delete(c *fiber.Ctx) error {
	idStr := c.Params("additionalClaimsID")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return badRequest(c, "invalid additionalClaimsID")
	}
	if err := h.store.Delete(strconv.FormatUint(id, 10)); err != nil {
		return notFound(c, "additional claim not found")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// lifetimeHandlers groups handlers for lifetime endpoints.
type lifetimeHandlers struct {
	kv smodel.KeyValueStore
}

func (h *lifetimeHandlers) get(c *fiber.Ctx) error {
	var seconds int
	found, err := h.kv.GetAs(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyLifetime, &seconds)
	if err != nil {
		return serverError(c, err.Error())
	}
	if !found || seconds <= 0 {
		seconds = int(storage.DefaultEntityConfigurationLifetime.Seconds())
	}
	return c.JSON(seconds)
}

func (h *lifetimeHandlers) put(c *fiber.Ctx) error {
	if len(c.Body()) == 0 {
		return badRequest(c, "empty body")
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(string(c.Body())))
	if err != nil {
		return badRequest(c, "invalid body: expected integer")
	}
	if seconds < 0 {
		return badRequest(c, "lifetime must be non-negative")
	}
	if err := h.kv.SetAny(
		smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyLifetime, seconds,
	); err != nil {
		return serverError(c, err.Error())
	}
	return c.JSON(seconds)
}

// metadataHandlers groups handlers for metadata endpoints.
type metadataHandlers struct {
	store *metadataStore
	kv    smodel.KeyValueStore
}

func (h *metadataHandlers) getAll(c *fiber.Ctx) error {
	rawAll, err := h.kv.Get(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata)
	if err != nil {
		return serverError(c, err.Error())
	}
	if rawAll == nil {
		return c.JSON(fiber.Map{})
	}
	var meta oidfed.Metadata
	if err := json.Unmarshal(rawAll, &meta); err != nil {
		return serverError(c, "invalid stored metadata")
	}
	return c.JSON(meta)
}

func (h *metadataHandlers) putAll(c *fiber.Ctx) error {
	var meta oidfed.Metadata
	if err := c.BodyParser(&meta); err != nil {
		return badRequest(c, "invalid body")
	}
	buf, _ := json.Marshal(meta)
	if err := h.kv.Set(smodel.KeyValueScopeEntityConfiguration, smodel.KeyValueKeyMetadata, buf); err != nil {
		return serverError(c, err.Error())
	}
	return c.JSON(meta)
}

func (h *metadataHandlers) getClaim(c *fiber.Ctx) error {
	entityType := c.Params("entityType")
	claim := c.Params("claim")

	meta, err := h.store.load()
	if err != nil {
		return serverError(c, err.Error())
	}
	if m, ok := meta[entityType]; ok {
		if v, ok := m[claim]; ok {
			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
			return c.Send(v)
		}
	}
	return notFound(c, "metadata not found")
}

func (h *metadataHandlers) putClaim(c *fiber.Ctx) error {
	entityType := c.Params("entityType")
	claim := c.Params("claim")

	if len(c.Body()) == 0 {
		return badRequest(c, "empty body")
	}

	meta, err := h.store.load()
	if err != nil {
		return serverError(c, err.Error())
	}
	if _, ok := meta[entityType]; !ok {
		meta[entityType] = make(map[string]json.RawMessage)
	}
	meta[entityType][claim] = c.Body()

	if err := h.store.save(meta); err != nil {
		return serverError(c, err.Error())
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	return c.Send(c.Body())
}

func (h *metadataHandlers) deleteClaim(c *fiber.Ctx) error {
	entityType := c.Params("entityType")
	claim := c.Params("claim")

	meta, err := h.store.load()
	if err != nil {
		return serverError(c, err.Error())
	}
	if m, ok := meta[entityType]; ok {
		delete(m, claim)
		if len(m) == 0 {
			delete(meta, entityType)
		}
		if err := h.store.save(meta); err != nil {
			return serverError(c, err.Error())
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *metadataHandlers) getEntityType(c *fiber.Ctx) error {
	entityType := c.Params("entityType")

	meta, err := h.store.load()
	if err != nil {
		return serverError(c, err.Error())
	}
	claims := meta[entityType]
	if claims == nil {
		claims = map[string]json.RawMessage{}
	}
	return c.JSON(claims)
}

func (h *metadataHandlers) putEntityType(c *fiber.Ctx) error {
	entityType := c.Params("entityType")
	var body map[string]json.RawMessage
	if err := c.BodyParser(&body); err != nil {
		return badRequest(c, "invalid body")
	}

	meta, err := h.store.load()
	if err != nil {
		return serverError(c, err.Error())
	}
	meta[entityType] = body

	if err := h.store.save(meta); err != nil {
		return serverError(c, err.Error())
	}
	return c.JSON(body)
}

func (h *metadataHandlers) postEntityType(c *fiber.Ctx) error {
	entityType := c.Params("entityType")
	var body map[string]json.RawMessage
	if err := c.BodyParser(&body); err != nil {
		return badRequest(c, "invalid body")
	}

	meta, err := h.store.load()
	if err != nil {
		return serverError(c, err.Error())
	}
	if _, ok := meta[entityType]; !ok {
		meta[entityType] = make(map[string]json.RawMessage)
	}
	for claim, raw := range body {
		meta[entityType][claim] = raw
	}

	if err := h.store.save(meta); err != nil {
		return serverError(c, err.Error())
	}
	return c.JSON(body)
}

func (h *metadataHandlers) deleteEntityType(c *fiber.Ctx) error {
	entityType := c.Params("entityType")

	meta, err := h.store.load()
	if err != nil {
		return serverError(c, err.Error())
	}
	if _, ok := meta[entityType]; !ok {
		return c.SendStatus(fiber.StatusNoContent)
	}
	delete(meta, entityType)

	if err := h.store.save(meta); err != nil {
		return serverError(c, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func registerEntityConfiguration(
	r fiber.Router, addClaimsStore smodel.AdditionalClaimsStore, kv smodel.KeyValueStore,
	fedEntity oidfed.FederationEntity,
) {
	g := r.Group("/entity-configuration")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)

	// Initialize handlers
	ecHandlers := &entityConfigHandlers{fedEntity: fedEntity}
	claimsHandlers := &additionalClaimsHandlers{store: addClaimsStore}
	ltHandlers := &lifetimeHandlers{kv: kv}
	metaStore := &metadataStore{kv: kv}
	metaHandlers := &metadataHandlers{
		store: metaStore,
		kv:    kv,
	}

	// Entity configuration
	g.Get("/", ecHandlers.get)

	// Additional claims
	g.Get("/additional-claims", claimsHandlers.list)
	withCacheWipe.Put("/additional-claims", claimsHandlers.set)
	withCacheWipe.Post("/additional-claims", claimsHandlers.create)
	g.Get("/additional-claims/:additionalClaimsID", claimsHandlers.get)
	withCacheWipe.Put("/additional-claims/:additionalClaimsID", claimsHandlers.update)
	withCacheWipe.Delete("/additional-claims/:additionalClaimsID", claimsHandlers.delete)

	// Lifetime
	g.Get("/lifetime", ltHandlers.get)
	withCacheWipe.Put("/lifetime", ltHandlers.put)

	// Metadata
	g.Get("/metadata", metaHandlers.getAll)
	withCacheWipe.Put("/metadata", metaHandlers.putAll)
	g.Get("/metadata/:entityType/:claim", metaHandlers.getClaim)
	withCacheWipe.Put("/metadata/:entityType/:claim", metaHandlers.putClaim)
	withCacheWipe.Delete("/metadata/:entityType/:claim", metaHandlers.deleteClaim)
	g.Get("/metadata/:entityType", metaHandlers.getEntityType)
	withCacheWipe.Put("/metadata/:entityType", metaHandlers.putEntityType)
	withCacheWipe.Post("/metadata/:entityType", metaHandlers.postEntityType)
	withCacheWipe.Delete("/metadata/:entityType", metaHandlers.deleteEntityType)
}

// isUniqueConstraintError performs a cheap check across supported drivers.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// SQLite
	if strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "constraint failed") {
		return true
	}
	// MySQL
	if strings.Contains(msg, "Duplicate entry") || strings.Contains(msg, "Error 1062") {
		return true
	}
	// Postgres
	if strings.Contains(msg, "duplicate key value") || strings.Contains(msg, "violates unique constraint") {
		return true
	}
	return false
}
