package adminapi

import (
	"encoding/json"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerSubordinateMetadata registers metadata endpoints for subordinates.
func registerSubordinateMetadata(r fiber.Router, subordinates model.SubordinateStorageBackend) {
	g := r.Group("/subordinates/:subordinateID/metadata")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - Get full subordinate-specific metadata
	g.Get("/", handleGetSubordinateMetadata(subordinates))

	// PUT / - Replace full subordinate-specific metadata
	withCacheWipe.Put("/", handlePutSubordinateMetadata(subordinates))

	// Entity type endpoints
	g.Get("/:entityType", handleGetSubordinateMetadataEntityType(subordinates))
	withCacheWipe.Put("/:entityType", handlePutSubordinateMetadataEntityType(subordinates))
	withCacheWipe.Post("/:entityType", handlePostSubordinateMetadataEntityType(subordinates))
	withCacheWipe.Delete("/:entityType", handleDeleteSubordinateMetadataEntityType(subordinates))

	// Claim endpoints
	g.Get("/:entityType/:claim", handleGetSubordinateMetadataClaim(subordinates))
	withCacheWipe.Put("/:entityType/:claim", handlePutSubordinateMetadataClaim(subordinates))
	withCacheWipe.Delete("/:entityType/:claim", handleDeleteSubordinateMetadataClaim(subordinates))
}

func handleGetSubordinateMetadata(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.Metadata == nil {
			return writeNotFound(c, "metadata not found")
		}
		return c.JSON(info.Metadata)
	}
}

func handlePutSubordinateMetadata(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		var body oidfed.Metadata
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		info.Metadata = &body
		if err := subordinates.Update(info.EntityID, *info); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(body)
	}
}

func handleGetSubordinateMetadataEntityType(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		m := getEntityMetadata(info.Metadata, et)
		if m == nil {
			return writeNotFound(c, "metadata not found")
		}
		return c.JSON(m)
	}
}

func handlePutSubordinateMetadataEntityType(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		var body map[string]any
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return writeBadBody(c)
		}
		if info.Metadata == nil {
			info.Metadata = &oidfed.Metadata{}
		}
		setEntityMetadata(info.Metadata, et, body)
		if err := subordinates.Update(info.EntityID, *info); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(body)
	}
}

func handlePostSubordinateMetadataEntityType(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		var body map[string]any
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return writeBadBody(c)
		}
		if info.Metadata == nil {
			info.Metadata = &oidfed.Metadata{}
		}
		existing := getEntityMetadata(info.Metadata, et)
		if existing == nil {
			existing = map[string]any{}
		}
		for k, v := range body {
			existing[k] = v
		}
		setEntityMetadata(info.Metadata, et, existing)
		if err := subordinates.Update(info.EntityID, *info); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(existing)
	}
}

func handleDeleteSubordinateMetadataEntityType(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if getEntityMetadata(info.Metadata, et) == nil {
			return writeNotFound(c, "metadata not found")
		}
		if info.Metadata == nil {
			info.Metadata = &oidfed.Metadata{}
		}
		deleteEntityMetadata(info.Metadata, et)
		if err := subordinates.Update(info.EntityID, *info); err != nil {
			return writeServerError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetSubordinateMetadataClaim(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		m := getEntityMetadata(info.Metadata, et)
		if m == nil {
			return writeNotFound(c, "metadata not found")
		}
		v, ok := m[claim]
		if !ok {
			return writeNotFound(c, "metadata not found")
		}
		return c.JSON(v)
	}
}

func handlePutSubordinateMetadataClaim(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		var body any
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return writeBadBody(c)
		}
		created := false
		if info.Metadata == nil {
			info.Metadata = &oidfed.Metadata{}
		}
		m := getEntityMetadata(info.Metadata, et)
		if m == nil {
			m = map[string]any{}
			created = true
		}
		if _, ok := m[claim]; !ok {
			created = true
		}
		m[claim] = body
		setEntityMetadata(info.Metadata, et, m)
		if err := subordinates.Update(info.EntityID, *info); err != nil {
			return writeServerError(c, err)
		}
		if created {
			return c.Status(fiber.StatusCreated).JSON(body)
		}
		return c.JSON(body)
	}
}

func handleDeleteSubordinateMetadataClaim(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		m := getEntityMetadata(info.Metadata, et)
		if m == nil {
			return writeNotFound(c, "metadata not found")
		}
		if _, ok := m[claim]; !ok {
			return writeNotFound(c, "metadata not found")
		}
		delete(m, claim)
		if info.Metadata == nil {
			info.Metadata = &oidfed.Metadata{}
		}
		setEntityMetadata(info.Metadata, et, m)
		if err := subordinates.Update(info.EntityID, *info); err != nil {
			return writeServerError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}
