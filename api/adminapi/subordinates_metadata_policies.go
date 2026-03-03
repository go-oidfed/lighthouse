package adminapi

import (
	"encoding/json"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerGeneralMetadataPolicies registers general metadata policy endpoints (no subordinateID).
func registerGeneralMetadataPolicies(r fiber.Router, kv model.KeyValueStore) {
	g := r.Group("/subordinates/metadata-policies")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - Get full policy
	g.Get("/", handleGetGeneralMetadataPolicy(kv))

	// PUT / - Replace full policy
	withCacheWipe.Put("/", handlePutGeneralMetadataPolicy(kv))

	// Entity type endpoints
	g.Get("/:entityType", handleGetGeneralMetadataPolicyEntityType(kv))
	withCacheWipe.Put("/:entityType", handlePutGeneralMetadataPolicyEntityType(kv))
	withCacheWipe.Post("/:entityType", handlePostGeneralMetadataPolicyEntityType(kv))
	withCacheWipe.Delete("/:entityType", handleDeleteGeneralMetadataPolicyEntityType(kv))

	// Claim endpoints
	g.Get("/:entityType/:claim", handleGetGeneralMetadataPolicyClaim(kv))
	withCacheWipe.Put("/:entityType/:claim", handlePutGeneralMetadataPolicyClaim(kv))
	withCacheWipe.Post("/:entityType/:claim", handlePostGeneralMetadataPolicyClaim(kv))
	withCacheWipe.Delete("/:entityType/:claim", handleDeleteGeneralMetadataPolicyClaim(kv))

	// Operator endpoints
	g.Get("/:entityType/:claim/:operator", handleGetGeneralMetadataPolicyOperator(kv))
	withCacheWipe.Put("/:entityType/:claim/:operator", handlePutGeneralMetadataPolicyOperator(kv))
	withCacheWipe.Post("/:entityType/:claim/:operator", handlePostGeneralMetadataPolicyOperator(kv))
	withCacheWipe.Delete("/:entityType/:claim/:operator", handleDeleteGeneralMetadataPolicyOperator(kv))
}

// loadGeneralMetadataPolicy loads the general metadata policy from KV store.
func loadGeneralMetadataPolicy(kv model.KeyValueStore) (*oidfed.MetadataPolicies, bool, error) {
	var mp oidfed.MetadataPolicies
	found, err := kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return &mp, false, nil
	}
	return &mp, true, nil
}

// saveGeneralMetadataPolicy saves the general metadata policy to KV store.
func saveGeneralMetadataPolicy(kv model.KeyValueStore, mp *oidfed.MetadataPolicies) error {
	return kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, *mp)
}

func handleGetGeneralMetadataPolicy(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		mp, _, err := loadGeneralMetadataPolicy(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(mp)
	}
}

func handlePutGeneralMetadataPolicy(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var mp oidfed.MetadataPolicies
		if err := c.BodyParser(&mp); err != nil {
			return writeBadBody(c)
		}
		if err := saveGeneralMetadataPolicy(kv, &mp); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(mp)
	}
}

func handleGetGeneralMetadataPolicyEntityType(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		mp, found, err := loadGeneralMetadataPolicy(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found {
			return c.JSON(oidfed.MetadataPolicy{})
		}
		policy := getMetadataPolicy(mp, et)
		if policy == nil {
			policy = oidfed.MetadataPolicy{}
		}
		return c.JSON(policy)
	}
}

func handlePutGeneralMetadataPolicyEntityType(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		var body oidfed.MetadataPolicy
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		mp, _, err := loadGeneralMetadataPolicy(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		setMetadataPolicy(mp, et, body)
		if err = saveGeneralMetadataPolicy(kv, mp); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(body)
	}
}

func handlePostGeneralMetadataPolicyEntityType(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		var body oidfed.MetadataPolicy
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		mp, _, err := loadGeneralMetadataPolicy(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		existing := getMetadataPolicy(mp, et)
		if existing == nil {
			existing = oidfed.MetadataPolicy{}
		}
		for claim, ops := range body {
			existing[claim] = ops
		}
		setMetadataPolicy(mp, et, existing)
		if err = saveGeneralMetadataPolicy(kv, mp); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(existing)
	}
}

func handleDeleteGeneralMetadataPolicyEntityType(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		mp, _, err := loadGeneralMetadataPolicy(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		deleteMetadataPolicy(mp, et)
		_ = saveGeneralMetadataPolicy(kv, mp)
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetGeneralMetadataPolicyClaim(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		mp, found, err := loadGeneralMetadataPolicy(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found {
			return writeNotFound(c, "metadata policy not found")
		}
		policy := getMetadataPolicy(mp, et)
		if policy == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		ops := policy[claim]
		if ops == nil {
			ops = oidfed.MetadataPolicyEntry{}
		}
		return c.JSON(ops)
	}
}

func handlePutGeneralMetadataPolicyClaim(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		var body oidfed.MetadataPolicyEntry
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		mp, _, err := loadGeneralMetadataPolicy(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		setMetadataPolicyEntry(mp, et, claim, body)
		if err := saveGeneralMetadataPolicy(kv, mp); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(body)
	}
}

func handlePostGeneralMetadataPolicyClaim(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		var body oidfed.MetadataPolicyEntry
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		mp, _, err := loadGeneralMetadataPolicy(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		existing := getMetadataPolicyEntry(mp, et, claim)
		if existing == nil {
			existing = oidfed.MetadataPolicyEntry{}
		}
		for op, val := range body {
			existing[op] = val
		}
		setMetadataPolicyEntry(mp, et, claim, existing)
		if err := saveGeneralMetadataPolicy(kv, mp); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(existing)
	}
}

func handleDeleteGeneralMetadataPolicyClaim(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		var mp map[string]map[string]map[string]any
		_, _ = kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
		if mp != nil {
			if m := mp[et]; m != nil {
				delete(m, claim)
				if len(m) == 0 {
					delete(mp, et)
				}
				_ = kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp)
			}
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetGeneralMetadataPolicyOperator(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		op := oidfed.PolicyOperatorName(c.Params("operator"))
		mp, found, err := loadGeneralMetadataPolicy(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found {
			return writeNotFound(c, "metadata policy not found")
		}
		entry := getMetadataPolicyEntry(mp, et, claim)
		if entry == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		val, ok := entry[op]
		if !ok {
			return writeNotFound(c, "operator not found")
		}
		return c.JSON(val)
	}
}

func handlePutGeneralMetadataPolicyOperator(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		op := oidfed.PolicyOperatorName(c.Params("operator"))
		var val any
		if err := c.BodyParser(&val); err != nil {
			return writeBadBody(c)
		}
		mp, _, err := loadGeneralMetadataPolicy(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		entry := getMetadataPolicyEntry(mp, et, claim)
		created := false
		if entry == nil {
			entry = oidfed.MetadataPolicyEntry{}
			created = true
		} else if _, ok := entry[op]; !ok {
			created = true
		}
		entry[op] = val
		setMetadataPolicyEntry(mp, et, claim, entry)
		if err := saveGeneralMetadataPolicy(kv, mp); err != nil {
			return writeServerError(c, err)
		}
		status := fiber.StatusOK
		if created {
			status = fiber.StatusCreated
		}
		return c.Status(status).JSON(val)
	}
}

func handlePostGeneralMetadataPolicyOperator(kv model.KeyValueStore) fiber.Handler {
	return handlePutGeneralMetadataPolicyOperator(kv)
}

func handleDeleteGeneralMetadataPolicyOperator(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		op := oidfed.PolicyOperatorName(c.Params("operator"))
		var mp map[string]map[string]map[string]any
		_, _ = kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
		if mp != nil {
			if m := mp[et]; m != nil {
				if ops := m[claim]; ops != nil {
					delete(ops, string(op))
					if len(ops) == 0 {
						delete(m, claim)
					}
					if len(m) == 0 {
						delete(mp, et)
					}
					_ = kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp)
				}
			}
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// registerSubordinateMetadataPolicies registers subordinate-specific metadata policy endpoints.
// All write operations are wrapped in transactions for atomicity.
func registerSubordinateMetadataPolicies(
	r fiber.Router,
	storages model.Backends,
) {
	g := r.Group("/subordinates/:subordinateID/metadata-policies")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - Get full subordinate-specific policies
	g.Get("/", handleGetSubordinateMetadataPolicies(storages.Subordinates))

	// PUT / - Replace full subordinate-specific policies (transactional)
	withCacheWipe.Put("/", handlePutSubordinateMetadataPolicies(storages))

	// POST / - Copy general metadata policies to subordinate (transactional)
	withCacheWipe.Post("/", handlePostSubordinateMetadataPolicies(storages))

	// DELETE / - Delete subordinate-specific policies (transactional)
	withCacheWipe.Delete("/", handleDeleteSubordinateMetadataPolicies(storages))

	// Entity type endpoints
	g.Get("/:entityType", handleGetSubordinateMetadataPoliciesEntityType(storages.Subordinates))
	withCacheWipe.Put("/:entityType", handlePutSubordinateMetadataPoliciesEntityType(storages))
	withCacheWipe.Post("/:entityType", handlePostSubordinateMetadataPoliciesEntityType(storages))
	withCacheWipe.Delete("/:entityType", handleDeleteSubordinateMetadataPoliciesEntityType(storages))

	// Claim endpoints
	g.Get("/:entityType/:claim", handleGetSubordinateMetadataPoliciesClaim(storages.Subordinates))
	withCacheWipe.Put("/:entityType/:claim", handlePutSubordinateMetadataPoliciesClaim(storages))
	withCacheWipe.Post("/:entityType/:claim", handlePostSubordinateMetadataPoliciesClaim(storages))
	withCacheWipe.Delete("/:entityType/:claim", handleDeleteSubordinateMetadataPoliciesClaim(storages))

	// Operator endpoints
	g.Get("/:entityType/:claim/:operator", handleGetSubordinateMetadataPoliciesOperator(storages.Subordinates))
	withCacheWipe.Put("/:entityType/:claim/:operator", handlePutSubordinateMetadataPoliciesOperator(storages))
	withCacheWipe.Delete("/:entityType/:claim/:operator", handleDeleteSubordinateMetadataPoliciesOperator(storages))
}

func handleGetSubordinateMetadataPolicies(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.MetadataPolicy == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		return c.JSON(info.MetadataPolicy)
	}
}

func handlePutSubordinateMetadataPolicies(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var body oidfed.MetadataPolicies
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			info.MetadataPolicy = &body
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated)
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.JSON(body)
	}
}

func handlePostSubordinateMetadataPolicies(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var result *oidfed.MetadataPolicies

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			general, found, err := loadGeneralMetadataPolicy(tx.KV)
			if err != nil {
				return err
			}
			if !found || general == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			} else {
				copied := *general
				info.MetadataPolicy = &copied
			}
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			result = info.MetadataPolicy
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage("copied from general"))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	}
}

func handleDeleteSubordinateMetadataPolicies(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.MetadataPolicy == nil {
				return model.NotFoundError("metadata policy not found")
			}
			info.MetadataPolicy = nil
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyDeleted)
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetSubordinateMetadataPoliciesEntityType(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.MetadataPolicy == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		policy := getMetadataPolicy(info.MetadataPolicy, et)
		if policy == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		return c.JSON(policy)
	}
}

func handlePutSubordinateMetadataPoliciesEntityType(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		var body oidfed.MetadataPolicy
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			setMetadataPolicy(info.MetadataPolicy, et, body)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage("entity type: "+et))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.JSON(body)
	}
}

func handlePostSubordinateMetadataPoliciesEntityType(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		var body oidfed.MetadataPolicy
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}

		var result oidfed.MetadataPolicy
		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			existing := getMetadataPolicy(info.MetadataPolicy, et)
			if existing == nil {
				existing = oidfed.MetadataPolicy{}
			}
			for claim, ops := range body {
				existing[claim] = ops
			}
			setMetadataPolicy(info.MetadataPolicy, et, existing)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			result = existing
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage("entity type: "+et))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.JSON(result)
	}
}

func handleDeleteSubordinateMetadataPoliciesEntityType(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.MetadataPolicy == nil {
				return model.NotFoundError("metadata policy not found")
			}
			deleteMetadataPolicy(info.MetadataPolicy, et)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyDeleted, WithMessage("entity type: "+et))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetSubordinateMetadataPoliciesClaim(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.MetadataPolicy == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		policy := getMetadataPolicy(info.MetadataPolicy, et)
		if policy == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		ops := policy[claim]
		if ops == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		return c.JSON(ops)
	}
}

func handlePutSubordinateMetadataPoliciesClaim(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		claim := c.Params("claim")
		var body oidfed.MetadataPolicyEntry
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			setMetadataPolicyEntry(info.MetadataPolicy, et, claim, body)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage(et+"."+claim))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.JSON(body)
	}
}

func handlePostSubordinateMetadataPoliciesClaim(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		claim := c.Params("claim")
		var body oidfed.MetadataPolicyEntry
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}

		var result oidfed.MetadataPolicyEntry
		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			policy := getMetadataPolicy(info.MetadataPolicy, et)
			if policy == nil {
				policy = oidfed.MetadataPolicy{}
			}
			existing := policy[claim]
			if existing == nil {
				existing = oidfed.MetadataPolicyEntry{}
			}
			for op, v := range body {
				existing[op] = v
			}
			policy[claim] = existing
			setMetadataPolicy(info.MetadataPolicy, et, policy)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			result = existing
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage(et+"."+claim))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.JSON(result)
	}
}

func handleDeleteSubordinateMetadataPoliciesClaim(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		claim := c.Params("claim")

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.MetadataPolicy == nil {
				return model.NotFoundError("metadata policy not found")
			}
			policy := getMetadataPolicy(info.MetadataPolicy, et)
			if policy == nil {
				return model.NotFoundError("metadata policy not found")
			}
			if _, ok := policy[claim]; !ok {
				return model.NotFoundError("metadata policy not found")
			}
			delete(policy, claim)
			setMetadataPolicy(info.MetadataPolicy, et, policy)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyDeleted, WithMessage(et+"."+claim))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetSubordinateMetadataPoliciesOperator(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		op := oidfed.PolicyOperatorName(c.Params("operator"))
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.MetadataPolicy == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		policy := getMetadataPolicy(info.MetadataPolicy, et)
		if policy == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		entry := policy[claim]
		if entry == nil {
			return writeNotFound(c, "metadata policy not found")
		}
		v, ok := entry[op]
		if !ok {
			return writeNotFound(c, "metadata policy not found")
		}
		return c.JSON(v)
	}
}

func handlePutSubordinateMetadataPoliciesOperator(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		claim := c.Params("claim")
		op := oidfed.PolicyOperatorName(c.Params("operator"))
		var body any
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return writeBadBody(c)
		}

		var created bool
		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			created = false
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			policy := getMetadataPolicy(info.MetadataPolicy, et)
			if policy == nil {
				policy = oidfed.MetadataPolicy{}
				created = true
			}
			entry := policy[claim]
			if entry == nil {
				entry = oidfed.MetadataPolicyEntry{}
				created = true
			}
			if _, ok := entry[op]; !ok {
				created = true
			}
			entry[op] = body
			policy[claim] = entry
			setMetadataPolicy(info.MetadataPolicy, et, policy)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage(et+"."+claim+"."+string(op)))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		if created {
			return c.Status(fiber.StatusCreated).JSON(body)
		}
		return c.JSON(body)
	}
}

func handleDeleteSubordinateMetadataPoliciesOperator(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		claim := c.Params("claim")
		op := oidfed.PolicyOperatorName(c.Params("operator"))

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.MetadataPolicy == nil {
				return model.NotFoundError("metadata policy not found")
			}
			policy := getMetadataPolicy(info.MetadataPolicy, et)
			if policy == nil {
				return model.NotFoundError("metadata policy not found")
			}
			entry := policy[claim]
			if entry == nil {
				return model.NotFoundError("metadata policy not found")
			}
			if _, ok := entry[op]; !ok {
				return model.NotFoundError("metadata policy not found")
			}
			delete(entry, op)
			policy[claim] = entry
			if len(entry) == 0 {
				delete(policy, claim)
			}
			setMetadataPolicy(info.MetadataPolicy, et, policy)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyDeleted, WithMessage(et+"."+claim+"."+string(op)))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// registerSubordinateMetadataPolicyCrit registers general metadata policy crit endpoints.
func registerSubordinateMetadataPolicyCrit(r fiber.Router, kv model.KeyValueStore) {
	g := r.Group("/subordinates/metadata-policy-crit")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - List all critical metadata policy operators
	g.Get("/", handleGetMetadataPolicyCrit(kv))

	// PUT / - Replace all critical metadata policy operators
	withCacheWipe.Put("/", handlePutMetadataPolicyCrit(kv))

	// POST / - Add a critical metadata policy operator
	withCacheWipe.Post("/", handlePostMetadataPolicyCrit(kv))

	// DELETE /:operator - Remove a critical metadata policy operator
	withCacheWipe.Delete("/:operator", handleDeleteMetadataPolicyCrit(kv))
}

func loadMetadataPolicyCrit(kv model.KeyValueStore) ([]string, error) {
	var operators []string
	found, err := kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicyCrit, &operators)
	if err != nil {
		return nil, err
	}
	if !found {
		return []string{}, nil
	}
	return operators, nil
}

func saveMetadataPolicyCrit(kv model.KeyValueStore, operators []string) error {
	return kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicyCrit, operators)
}

func handleGetMetadataPolicyCrit(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		operators, err := loadMetadataPolicyCrit(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(operators)
	}
}

func handlePutMetadataPolicyCrit(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var operators []string
		if err := c.BodyParser(&operators); err != nil {
			return writeBadBody(c)
		}
		if err := saveMetadataPolicyCrit(kv, operators); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(operators)
	}
}

func handlePostMetadataPolicyCrit(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var operator string
		if err := c.BodyParser(&operator); err != nil {
			return writeBadBody(c)
		}
		operators, err := loadMetadataPolicyCrit(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		for _, op := range operators {
			if op == operator {
				return writeConflict(c, "operator already exists")
			}
		}
		operators = append(operators, operator)
		if err := saveMetadataPolicyCrit(kv, operators); err != nil {
			return writeServerError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(operator)
	}
}

func handleDeleteMetadataPolicyCrit(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		operator := c.Params("operator")
		operators, err := loadMetadataPolicyCrit(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		found := -1
		for i, op := range operators {
			if op == operator {
				found = i
				break
			}
		}
		if found == -1 {
			return writeNotFound(c, "operator not found")
		}
		operators = append(operators[:found], operators[found+1:]...)
		if err := saveMetadataPolicyCrit(kv, operators); err != nil {
			return writeServerError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// filterUsedPolicyOperators returns only the operators from configuredCrit that are actually
// used in the given metadata policy.
func filterUsedPolicyOperators(mp *oidfed.MetadataPolicies, configuredCrit []oidfed.PolicyOperatorName) []oidfed.PolicyOperatorName {
	if mp == nil || len(configuredCrit) == 0 {
		return nil
	}

	// Collect all operators used in the metadata policy
	usedOperators := make(map[oidfed.PolicyOperatorName]bool)
	collectOperatorsFromPolicy := func(policy oidfed.MetadataPolicy) {
		if policy == nil {
			return
		}
		for _, entry := range policy {
			for op := range entry {
				usedOperators[op] = true
			}
		}
	}

	// Check all standard entity type policies
	collectOperatorsFromPolicy(mp.OpenIDProvider)
	collectOperatorsFromPolicy(mp.RelyingParty)
	collectOperatorsFromPolicy(mp.OAuthAuthorizationServer)
	collectOperatorsFromPolicy(mp.OAuthClient)
	collectOperatorsFromPolicy(mp.OAuthProtectedResource)
	collectOperatorsFromPolicy(mp.FederationEntity)

	// Check extra policies
	for _, policy := range mp.Extra {
		collectOperatorsFromPolicy(policy)
	}

	// Filter configured crit operators to only those actually used
	var result []oidfed.PolicyOperatorName
	for _, op := range configuredCrit {
		if usedOperators[op] {
			result = append(result, op)
		}
	}
	return result
}
