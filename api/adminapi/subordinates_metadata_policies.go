package adminapi

import (
	"encoding/json"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// generalMetadataPolicyStore wraps KV operations for general metadata policies.
type generalMetadataPolicyStore struct {
	kv model.KeyValueStore
}

func (s *generalMetadataPolicyStore) load() (*oidfed.MetadataPolicies, bool, error) {
	var mp oidfed.MetadataPolicies
	found, err := s.kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return &mp, false, nil
	}
	return &mp, true, nil
}

func (s *generalMetadataPolicyStore) save(mp *oidfed.MetadataPolicies) error {
	return s.kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, *mp)
}

func (s *generalMetadataPolicyStore) loadRaw() (map[string]map[string]map[string]any, error) {
	var mp map[string]map[string]map[string]any
	_, _ = s.kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
	return mp, nil
}

func (s *generalMetadataPolicyStore) saveRaw(mp map[string]map[string]map[string]any) error {
	return s.kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp)
}

// generalPolicyHandlers groups handlers for general metadata policy endpoints.
type generalPolicyHandlers struct {
	store *generalMetadataPolicyStore
}

func (h *generalPolicyHandlers) getAll(c *fiber.Ctx) error {
	mp, _, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(mp)
}

func (h *generalPolicyHandlers) putAll(c *fiber.Ctx) error {
	var mp oidfed.MetadataPolicies
	if err := c.BodyParser(&mp); err != nil {
		return writeBadBody(c)
	}
	if err := h.store.save(&mp); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(mp)
}

func (h *generalPolicyHandlers) getEntityType(c *fiber.Ctx) error {
	et := c.Params("entityType")
	mp, found, err := h.store.load()
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

func (h *generalPolicyHandlers) putEntityType(c *fiber.Ctx) error {
	et := c.Params("entityType")
	var body oidfed.MetadataPolicy
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}
	mp, _, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	setMetadataPolicy(mp, et, body)
	if err = h.store.save(mp); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(body)
}

func (h *generalPolicyHandlers) postEntityType(c *fiber.Ctx) error {
	et := c.Params("entityType")
	var body oidfed.MetadataPolicy
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}
	mp, _, err := h.store.load()
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
	if err = h.store.save(mp); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(existing)
}

func (h *generalPolicyHandlers) deleteEntityType(c *fiber.Ctx) error {
	et := c.Params("entityType")
	mp, _, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	deleteMetadataPolicy(mp, et)
	_ = h.store.save(mp)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *generalPolicyHandlers) getClaim(c *fiber.Ctx) error {
	et := c.Params("entityType")
	claim := c.Params("claim")
	mp, found, err := h.store.load()
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

func (h *generalPolicyHandlers) putClaim(c *fiber.Ctx) error {
	et := c.Params("entityType")
	claim := c.Params("claim")
	var body oidfed.MetadataPolicyEntry
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}
	mp, _, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	setMetadataPolicyEntry(mp, et, claim, body)
	if err := h.store.save(mp); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(body)
}

func (h *generalPolicyHandlers) postClaim(c *fiber.Ctx) error {
	et := c.Params("entityType")
	claim := c.Params("claim")
	var body oidfed.MetadataPolicyEntry
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}
	mp, _, err := h.store.load()
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
	if err := h.store.save(mp); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(existing)
}

func (h *generalPolicyHandlers) deleteClaim(c *fiber.Ctx) error {
	et := c.Params("entityType")
	claim := c.Params("claim")
	mp, _ := h.store.loadRaw()
	if mp != nil {
		if m := mp[et]; m != nil {
			delete(m, claim)
			if len(m) == 0 {
				delete(mp, et)
			}
			_ = h.store.saveRaw(mp)
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *generalPolicyHandlers) getOperator(c *fiber.Ctx) error {
	et := c.Params("entityType")
	claim := c.Params("claim")
	op := oidfed.PolicyOperatorName(c.Params("operator"))
	mp, found, err := h.store.load()
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

func (h *generalPolicyHandlers) putOperator(c *fiber.Ctx) error {
	et := c.Params("entityType")
	claim := c.Params("claim")
	op := oidfed.PolicyOperatorName(c.Params("operator"))
	var val any
	if err := c.BodyParser(&val); err != nil {
		return writeBadBody(c)
	}
	mp, _, err := h.store.load()
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
	if err := h.store.save(mp); err != nil {
		return writeServerError(c, err)
	}
	status := fiber.StatusOK
	if created {
		status = fiber.StatusCreated
	}
	return c.Status(status).JSON(val)
}

func (h *generalPolicyHandlers) deleteOperator(c *fiber.Ctx) error {
	et := c.Params("entityType")
	claim := c.Params("claim")
	op := oidfed.PolicyOperatorName(c.Params("operator"))
	mp, _ := h.store.loadRaw()
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
				_ = h.store.saveRaw(mp)
			}
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// subordinatePolicyHandlers groups handlers for subordinate-specific metadata policy endpoints.
type subordinatePolicyHandlers struct {
	storages model.Backends
}

func (h *subordinatePolicyHandlers) getAll(c *fiber.Ctx) error {
	info, ok := handleSubordinateLookup(c, h.storages.Subordinates)
	if !ok {
		return nil
	}
	if info.MetadataPolicy == nil {
		return writeNotFound(c, "metadata policy not found")
	}
	return c.JSON(info.MetadataPolicy)
}

func (h *subordinatePolicyHandlers) putAll(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	var body oidfed.MetadataPolicies
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}

	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
			}
			info.MetadataPolicy = &body
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated)
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	return c.JSON(body)
}

func (h *subordinatePolicyHandlers) postAll(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	var result *oidfed.MetadataPolicies
	store := &generalMetadataPolicyStore{kv: h.storages.KV}

	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
			}
			general, found, err := store.load()
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
			return RecordEvent(
				tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage("copied from general"),
			)
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *subordinatePolicyHandlers) deleteAll(c *fiber.Ctx) error {
	id := c.Params("subordinateID")

	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
			}
			if info.MetadataPolicy == nil {
				return model.NotFoundError("metadata policy not found")
			}
			info.MetadataPolicy = nil
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyDeleted)
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *subordinatePolicyHandlers) getEntityType(c *fiber.Ctx) error {
	et := c.Params("entityType")
	info, ok := handleSubordinateLookup(c, h.storages.Subordinates)
	if !ok {
		return nil
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

func (h *subordinatePolicyHandlers) putEntityType(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	et := c.Params("entityType")
	var body oidfed.MetadataPolicy
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}

	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
			}
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			setMetadataPolicy(info.MetadataPolicy, et, body)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(
				tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage("entity type: "+et),
			)
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	return c.JSON(body)
}

func (h *subordinatePolicyHandlers) postEntityType(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	et := c.Params("entityType")
	var body oidfed.MetadataPolicy
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}

	var result oidfed.MetadataPolicy
	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
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
			return RecordEvent(
				tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage("entity type: "+et),
			)
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	return c.JSON(result)
}

func (h *subordinatePolicyHandlers) deleteEntityType(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	et := c.Params("entityType")

	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
			}
			if info.MetadataPolicy == nil {
				return model.NotFoundError("metadata policy not found")
			}
			deleteMetadataPolicy(info.MetadataPolicy, et)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(
				tx.SubordinateEvents, info.ID, model.EventTypePolicyDeleted, WithMessage("entity type: "+et),
			)
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *subordinatePolicyHandlers) getClaim(c *fiber.Ctx) error {
	et := c.Params("entityType")
	claim := c.Params("claim")
	info, ok := handleSubordinateLookup(c, h.storages.Subordinates)
	if !ok {
		return nil
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

func (h *subordinatePolicyHandlers) putClaim(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	et := c.Params("entityType")
	claim := c.Params("claim")
	var body oidfed.MetadataPolicyEntry
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}

	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
			}
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			setMetadataPolicyEntry(info.MetadataPolicy, et, claim, body)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage(et+"."+claim))
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	return c.JSON(body)
}

func (h *subordinatePolicyHandlers) postClaim(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	et := c.Params("entityType")
	claim := c.Params("claim")
	var body oidfed.MetadataPolicyEntry
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}

	var result oidfed.MetadataPolicyEntry
	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
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
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	return c.JSON(result)
}

func (h *subordinatePolicyHandlers) deleteClaim(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	et := c.Params("entityType")
	claim := c.Params("claim")

	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
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
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *subordinatePolicyHandlers) getOperator(c *fiber.Ctx) error {
	et := c.Params("entityType")
	claim := c.Params("claim")
	op := oidfed.PolicyOperatorName(c.Params("operator"))
	info, ok := handleSubordinateLookup(c, h.storages.Subordinates)
	if !ok {
		return nil
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

func (h *subordinatePolicyHandlers) putOperator(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	et := c.Params("entityType")
	claim := c.Params("claim")
	op := oidfed.PolicyOperatorName(c.Params("operator"))
	var body any
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return writeBadBody(c)
	}

	var created bool
	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
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
			return RecordEvent(
				tx.SubordinateEvents, info.ID, model.EventTypePolicyUpdated, WithMessage(et+"."+claim+"."+string(op)),
			)
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	if created {
		return c.Status(fiber.StatusCreated).JSON(body)
	}
	return c.JSON(body)
}

func (h *subordinatePolicyHandlers) deleteOperator(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	et := c.Params("entityType")
	claim := c.Params("claim")
	op := oidfed.PolicyOperatorName(c.Params("operator"))

	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			info, err := getSubordinateByDBID(tx.Subordinates, id)
			if err != nil {
				return err
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
			return RecordEvent(
				tx.SubordinateEvents, info.ID, model.EventTypePolicyDeleted, WithMessage(et+"."+claim+"."+string(op)),
			)
		},
	)
	if err != nil {
		return handleTxError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// metadataPolicyCritHandlers groups handlers for metadata policy crit endpoints.
type metadataPolicyCritHandlers struct {
	kv model.KeyValueStore
}

func (h *metadataPolicyCritHandlers) load() ([]string, error) {
	var operators []string
	found, err := h.kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicyCrit, &operators)
	if err != nil {
		return nil, err
	}
	if !found {
		return []string{}, nil
	}
	return operators, nil
}

func (h *metadataPolicyCritHandlers) save(operators []string) error {
	return h.kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicyCrit, operators)
}

func (h *metadataPolicyCritHandlers) getAll(c *fiber.Ctx) error {
	operators, err := h.load()
	if err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(operators)
}

func (h *metadataPolicyCritHandlers) putAll(c *fiber.Ctx) error {
	var operators []string
	if err := c.BodyParser(&operators); err != nil {
		return writeBadBody(c)
	}
	if err := h.save(operators); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(operators)
}

func (h *metadataPolicyCritHandlers) post(c *fiber.Ctx) error {
	var operator string
	if err := c.BodyParser(&operator); err != nil {
		return writeBadBody(c)
	}
	operators, err := h.load()
	if err != nil {
		return writeServerError(c, err)
	}
	for _, op := range operators {
		if op == operator {
			return writeConflict(c, "operator already exists")
		}
	}
	operators = append(operators, operator)
	if err := h.save(operators); err != nil {
		return writeServerError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(operator)
}

func (h *metadataPolicyCritHandlers) delete(c *fiber.Ctx) error {
	operator := c.Params("operator")
	operators, err := h.load()
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
	if err := h.save(operators); err != nil {
		return writeServerError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// registerGeneralMetadataPolicies registers general metadata policy endpoints (no subordinateID).
func registerGeneralMetadataPolicies(r fiber.Router, kv model.KeyValueStore) {
	g := r.Group("/subordinates/metadata-policies")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	store := &generalMetadataPolicyStore{kv: kv}
	h := &generalPolicyHandlers{store: store}

	g.Get("/", h.getAll)
	withCacheWipe.Put("/", h.putAll)

	g.Get("/:entityType", h.getEntityType)
	withCacheWipe.Put("/:entityType", h.putEntityType)
	withCacheWipe.Post("/:entityType", h.postEntityType)
	withCacheWipe.Delete("/:entityType", h.deleteEntityType)

	g.Get("/:entityType/:claim", h.getClaim)
	withCacheWipe.Put("/:entityType/:claim", h.putClaim)
	withCacheWipe.Post("/:entityType/:claim", h.postClaim)
	withCacheWipe.Delete("/:entityType/:claim", h.deleteClaim)

	g.Get("/:entityType/:claim/:operator", h.getOperator)
	withCacheWipe.Put("/:entityType/:claim/:operator", h.putOperator)
	withCacheWipe.Post("/:entityType/:claim/:operator", h.putOperator) // POST delegates to PUT
	withCacheWipe.Delete("/:entityType/:claim/:operator", h.deleteOperator)
}

// registerSubordinateMetadataPolicies registers subordinate-specific metadata policy endpoints.
func registerSubordinateMetadataPolicies(r fiber.Router, storages model.Backends) {
	g := r.Group("/subordinates/:subordinateID/metadata-policies")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	h := &subordinatePolicyHandlers{storages: storages}

	g.Get("/", h.getAll)
	withCacheWipe.Put("/", h.putAll)
	withCacheWipe.Post("/", h.postAll)
	withCacheWipe.Delete("/", h.deleteAll)

	g.Get("/:entityType", h.getEntityType)
	withCacheWipe.Put("/:entityType", h.putEntityType)
	withCacheWipe.Post("/:entityType", h.postEntityType)
	withCacheWipe.Delete("/:entityType", h.deleteEntityType)

	g.Get("/:entityType/:claim", h.getClaim)
	withCacheWipe.Put("/:entityType/:claim", h.putClaim)
	withCacheWipe.Post("/:entityType/:claim", h.postClaim)
	withCacheWipe.Delete("/:entityType/:claim", h.deleteClaim)

	g.Get("/:entityType/:claim/:operator", h.getOperator)
	withCacheWipe.Put("/:entityType/:claim/:operator", h.putOperator)
	withCacheWipe.Delete("/:entityType/:claim/:operator", h.deleteOperator)
}

// registerSubordinateMetadataPolicyCrit registers general metadata policy crit endpoints.
func registerSubordinateMetadataPolicyCrit(r fiber.Router, kv model.KeyValueStore) {
	g := r.Group("/subordinates/metadata-policy-crit")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	h := &metadataPolicyCritHandlers{kv: kv}

	g.Get("/", h.getAll)
	withCacheWipe.Put("/", h.putAll)
	withCacheWipe.Post("/", h.post)
	withCacheWipe.Delete("/:operator", h.delete)
}

// filterUsedPolicyOperators returns only the operators from configuredCrit that are actually
// used in the given metadata policy.
func filterUsedPolicyOperators(
	mp *oidfed.MetadataPolicies, configuredCrit []oidfed.PolicyOperatorName,
) []oidfed.PolicyOperatorName {
	if mp == nil || len(configuredCrit) == 0 {
		return nil
	}

	usedOperators := make(map[oidfed.PolicyOperatorName]bool)
	collectOperators := func(policy oidfed.MetadataPolicy) {
		if policy == nil {
			return
		}
		for _, entry := range policy {
			for op := range entry {
				usedOperators[op] = true
			}
		}
	}

	collectOperators(mp.OpenIDProvider)
	collectOperators(mp.RelyingParty)
	collectOperators(mp.OAuthAuthorizationServer)
	collectOperators(mp.OAuthClient)
	collectOperators(mp.OAuthProtectedResource)
	collectOperators(mp.FederationEntity)

	for _, policy := range mp.Extra {
		collectOperators(policy)
	}

	var result []oidfed.PolicyOperatorName
	for _, op := range configuredCrit {
		if usedOperators[op] {
			result = append(result, op)
		}
	}
	return result
}
