package adminapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v3/jwk"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

var DefaultSubordinateStatus = model.StatusActive

func registerSubordinates(r fiber.Router, subordinates model.SubordinateStorageBackend) {
	g := r.Group("/subordinates")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	// List subordinates, optional filters via query: entity_type, status
	g.Get(
		"/", func(c *fiber.Ctx) error {
			type request struct {
				Status     *model.Status `query:"-"`
				EntityType []string      `query:"entity_type"`
			}
			var req request
			var infos []model.BasicSubordinateInfo
			var err error

			if err = c.QueryParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
			}

			s := c.Query("status")
			if s != "" {
				st, err := model.ParseStatus(s)
				if err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(
						oidfed.ErrorInvalidRequest(
							fmt.Sprintf(
								"invalid status: %s", err.Error(),
							),
						),
					)
				}
				req.Status = &st
			}

			if req.EntityType != nil {
				if req.Status != nil {
					infos, err = subordinates.GetByStatusAndAnyEntityType(*req.Status, req.EntityType)
				} else {
					// Only entity_type in query
					infos, err = subordinates.GetByAnyEntityType(req.EntityType)
				}
			} else if req.Status != nil {
				// Only status in query
				infos, err = subordinates.GetByStatus(*req.Status)
			} else {
				infos, err = subordinates.GetAll()
			}
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(infos)
		},
	)

	// Create subordinate
	g.Post(
		"/", func(c *fiber.Ctx) error {
			var req model.ExtendedSubordinateInfo
			req.Status = DefaultSubordinateStatus
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if req.EntityID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("missing entity_id"))
			}
			if err := subordinates.Add(req); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			stored, err := subordinates.Get(req.EntityID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(stored)
		},
	)

	// Get subordinate details
	g.Get(
		"/:subordinateID", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			fmt.Printf("INFO: %+v\n", info) //TODO remove
			return c.JSON(*info)
		},
	)

	// Update subordinate details (replace basic fields)
	withCacheWipe.Put(
		"/:subordinateID", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			existing, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if existing == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			var body struct {
				Description           *string  `json:"description"`
				RegisteredEntityTypes []string `json:"registered_entity_types"`
			}
			if err = c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if body.Description != nil {
				existing.Description = *body.Description
			}
			if body.RegisteredEntityTypes != nil {
				subordinateEntityTypes := make([]model.SubordinateEntityType, len(body.RegisteredEntityTypes))
				for i, et := range body.RegisteredEntityTypes {
					subordinateEntityTypes[i] = model.SubordinateEntityType{EntityType: et}
				}
				existing.SubordinateEntityTypes = subordinateEntityTypes
			}
			if err = subordinates.Update(existing.EntityID, *existing); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(existing)
		},
	)

	// Delete subordinate
	withCacheWipe.Delete(
		"/:subordinateID", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			if err := subordinates.DeleteByDBID(id); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Update subordinate status
	withCacheWipe.Put(
		"/:subordinateID/status", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			var status model.Status
			if err := c.BodyParser(&status); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if err := subordinates.UpdateStatusByDBID(id, status); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}

			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			return c.JSON(info)
		},
	)
	g.Get("/:subordinateID/statement", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"statement": fiber.Map{}}) })
	g.Get(
		"/:subordinateID/history", func(c *fiber.Ctx) error {
			// Placeholder: hook into events model.SubordinateEvent when available
			return c.JSON(fiber.Map{"events": []fiber.Map{}})
		},
	)

	// Subordinate additional claims - handlers registered separately in registerSubordinateAdditionalClaims
}

// General metadata policies (no subordinateID)
func registerGeneralMetadataPolicies(r fiber.Router, storagesKV model.KeyValueStore) {
	g := r.Group("/subordinates/metadata-policies")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	// Stored in KV at scope subordinate_statement, key metadata_policy
	// Data structure: oidfed.MetadataPolicies

	// Helpers to reduce duplication
	getPolicy := func(mp *oidfed.MetadataPolicies, et string) oidfed.MetadataPolicy {
		switch et {
		case "openid_provider":
			return mp.OpenIDProvider
		case "openid_relying_party":
			return mp.RelyingParty
		case "oauth_authorization_server":
			return mp.OAuthAuthorizationServer
		case "oauth_client":
			return mp.OAuthClient
		case "oauth_resource":
			return mp.OAuthProtectedResource
		case "federation_entity":
			return mp.FederationEntity
		default:
			if mp.Extra != nil {
				return mp.Extra[et]
			}
		}
		return nil
	}
	setPolicy := func(mp *oidfed.MetadataPolicies, et string, policy oidfed.MetadataPolicy) {
		switch et {
		case "openid_provider":
			mp.OpenIDProvider = policy
		case "openid_relying_party":
			mp.RelyingParty = policy
		case "oauth_authorization_server":
			mp.OAuthAuthorizationServer = policy
		case "oauth_client":
			mp.OAuthClient = policy
		case "oauth_resource":
			mp.OAuthProtectedResource = policy
		case "federation_entity":
			mp.FederationEntity = policy
		default:
			if mp.Extra == nil {
				mp.Extra = map[string]oidfed.MetadataPolicy{}
			}
			mp.Extra[et] = policy
		}
	}
	getEntry := func(mp *oidfed.MetadataPolicies, et, claim string) oidfed.MetadataPolicyEntry {
		policy := getPolicy(mp, et)
		if policy == nil {
			return nil
		}
		return policy[claim]
	}
	setEntry := func(mp *oidfed.MetadataPolicies, et, claim string, entry oidfed.MetadataPolicyEntry) {
		policy := getPolicy(mp, et)
		if policy == nil {
			policy = oidfed.MetadataPolicy{}
		}
		policy[claim] = entry
		setPolicy(mp, et, policy)
	}

	// GET full policy
	g.Get(
		"/", func(c *fiber.Ctx) error {
			var mp oidfed.MetadataPolicies
			_, err := storagesKV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp,
			)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			fmt.Println("PING")
			return c.JSON(mp)
		},
	)
	// PUT full policy
	withCacheWipe.Put(
		"/", func(c *fiber.Ctx) error {
			var mp oidfed.MetadataPolicies
			if err := c.BodyParser(&mp); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if err := storagesKV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(mp)
		},
	)
	// GET entity type
	g.Get(
		"/:entityType", func(c *fiber.Ctx) error {
			var mp oidfed.MetadataPolicies
			found, err := storagesKV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp,
			)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			et := c.Params("entityType")
			if !found {
				return c.JSON(oidfed.MetadataPolicy{})
			}
			policy := getPolicy(&mp, et)
			if policy == nil {
				policy = oidfed.MetadataPolicy{}
			}
			return c.JSON(policy)
		},
	)

	// PUT entity type (replace)
	withCacheWipe.Put(
		"/:entityType", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			var body oidfed.MetadataPolicy
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			var mp oidfed.MetadataPolicies
			_, err := storagesKV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			setPolicy(&mp, et, body)
			if err = storagesKV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(body)
		},
	)
	// POST entity type (merge)
	withCacheWipe.Post(
		"/:entityType", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			var body oidfed.MetadataPolicy
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			var mp oidfed.MetadataPolicies
			_, err := storagesKV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			existing := getPolicy(&mp, et)
			if existing == nil {
				existing = oidfed.MetadataPolicy{}
			}
			for claim, ops := range body {
				existing[claim] = ops
			}
			setPolicy(&mp, et, existing)
			if err = storagesKV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(existing)
		},
	)
	// DELETE entity type
	withCacheWipe.Delete(
		"/:entityType", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			var mp oidfed.MetadataPolicies
			_, err := storagesKV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if et == "openid_provider" {
				mp.OpenIDProvider = nil
			} else if et == "openid_relying_party" {
				mp.RelyingParty = nil
			} else if et == "oauth_authorization_server" {
				mp.OAuthAuthorizationServer = nil
			} else if et == "oauth_client" {
				mp.OAuthClient = nil
			} else if et == "oauth_resource" {
				mp.OAuthProtectedResource = nil
			} else if et == "federation_entity" {
				mp.FederationEntity = nil
			} else if mp.Extra != nil {
				delete(mp.Extra, et)
			}
			_ = storagesKV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp)
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
	// GET claim
	g.Get(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			claim := c.Params("claim")
			var mp oidfed.MetadataPolicies
			found, err := storagesKV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp,
			)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("metadata policy not found"))
			}
			policy := getPolicy(&mp, et)
			if policy == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("metadata policy not found"))
			}
			ops := policy[claim]
			if ops == nil {
				ops = oidfed.MetadataPolicyEntry{}
			}
			return c.JSON(ops)
		},
	)
	// PUT claim (replace)
	withCacheWipe.Put(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			claim := c.Params("claim")
			var body oidfed.MetadataPolicyEntry
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			var mp oidfed.MetadataPolicies
			_, err := storagesKV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			setEntry(&mp, et, claim, body)
			if err := storagesKV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(body)
		},
	)
	// POST claim (merge operators)
	withCacheWipe.Post(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			claim := c.Params("claim")
			var body oidfed.MetadataPolicyEntry
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			var mp oidfed.MetadataPolicies
			_, err := storagesKV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			existing := getEntry(&mp, et, claim)
			if existing == nil {
				existing = oidfed.MetadataPolicyEntry{}
			}
			for op, val := range body {
				existing[op] = val
			}
			setEntry(&mp, et, claim, existing)
			if err := storagesKV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(existing)
		},
	)
	// DELETE claim
	withCacheWipe.Delete(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			claim := c.Params("claim")
			var mp map[string]map[string]map[string]any
			_, _ = storagesKV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
			if mp != nil {
				if m := mp[et]; m != nil {
					delete(m, claim)
					if len(m) == 0 {
						delete(mp, et)
					}
					_ = storagesKV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp)
				}
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
	// GET operator value
	g.Get(
		"/:entityType/:claim/:operator", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			claim := c.Params("claim")
			op := oidfed.PolicyOperatorName(c.Params("operator"))
			var mp oidfed.MetadataPolicies
			found, err := storagesKV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp,
			)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("metadata policy not found"))
			}
			entry := getEntry(&mp, et, claim)
			if entry == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("metadata policy not found"))
			}
			val, ok := entry[op]
			if !ok {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("operator not found"))
			}
			return c.JSON(val)
		},
	)
	// PUT operator value (create/update)
	withCacheWipe.Put(
		"/:entityType/:claim/:operator", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			claim := c.Params("claim")
			op := oidfed.PolicyOperatorName(c.Params("operator"))
			var val any
			if err := c.BodyParser(&val); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			var mp oidfed.MetadataPolicies
			_, err := storagesKV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			entry := getEntry(&mp, et, claim)
			created := 0
			if entry == nil {
				entry = oidfed.MetadataPolicyEntry{}
				created = 1
			} else if _, ok := entry[op]; !ok {
				created = 1
			}
			entry[op] = val
			setEntry(&mp, et, claim, entry)
			if err := storagesKV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			status := fiber.StatusOK
			if created == 1 {
				status = fiber.StatusCreated
			}
			return c.Status(status).JSON(val)
		},
	)
	// POST operator value (same semantics as PUT)
	withCacheWipe.Post(
		"/:entityType/:claim/:operator", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			claim := c.Params("claim")
			op := oidfed.PolicyOperatorName(c.Params("operator"))
			var val any
			if err := c.BodyParser(&val); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			var mp oidfed.MetadataPolicies
			_, err := storagesKV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			entry := getEntry(&mp, et, claim)
			created := 0
			if entry == nil {
				entry = oidfed.MetadataPolicyEntry{}
				created = 1
			} else if _, ok := entry[op]; !ok {
				created = 1
			}
			entry[op] = val
			setEntry(&mp, et, claim, entry)
			if err := storagesKV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			status := fiber.StatusOK
			if created == 1 {
				status = fiber.StatusCreated
			}
			return c.Status(status).JSON(val)
		},
	)
	// DELETE operator value
	withCacheWipe.Delete(
		"/:entityType/:claim/:operator", func(c *fiber.Ctx) error {
			et := c.Params("entityType")
			claim := c.Params("claim")
			op := oidfed.PolicyOperatorName(c.Params("operator"))
			var mp map[string]map[string]map[string]any
			_, _ = storagesKV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
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
						_ = storagesKV.SetAny(
							model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp,
						)
					}
				}
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
}

func registerSubordinateMetadata(r fiber.Router, subordinates model.SubordinateStorageBackend) {
	g := r.Group("/subordinates/:subordinateID/metadata")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	getSub := func(dbID string) (*model.ExtendedSubordinateInfo, error) {
		info, err := subordinates.GetByDBID(dbID)
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, model.NotFoundError("subordinate not found")
		}
		return info, nil
	}
	writeNotFound := func(c *fiber.Ctx, msg string) error {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(msg))
	}
	writeServerError := func(c *fiber.Ctx, err error) error {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	writeBadBody := func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}

	getEntityMetadata := func(md *oidfed.Metadata, et string) map[string]any {
		if md == nil {
			return nil
		}
		if md.Extra == nil {
			return nil
		}
		v, ok := md.Extra[et]
		if !ok {
			return nil
		}
		// Metadata is modeled as arbitrary JSON in lib; in lighthouse we store it as map.
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		return m
	}
	setEntityMetadata := func(md *oidfed.Metadata, et string, m map[string]any) {
		if md.Extra == nil {
			md.Extra = map[string]any{}
		}
		md.Extra[et] = m
	}
	deleteEntityMetadata := func(md *oidfed.Metadata, et string) {
		if md == nil || md.Extra == nil {
			return
		}
		delete(md.Extra, et)
	}

	// GET full subordinate-specific metadata
	g.Get(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			if info.Metadata == nil {
				return writeNotFound(c, "metadata not found")
			}
			return c.JSON(info.Metadata)
		},
	)

	// PUT replace full subordinate-specific metadata
	withCacheWipe.Put(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
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
		},
	)

	// Entity type
	g.Get(
		"/:entityType", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			m := getEntityMetadata(info.Metadata, et)
			if m == nil {
				return writeNotFound(c, "metadata not found")
			}
			return c.JSON(m)
		},
	)
	withCacheWipe.Put(
		"/:entityType", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
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
		},
	)
	withCacheWipe.Post(
		"/:entityType", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
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
		},
	)
	withCacheWipe.Delete(
		"/:entityType", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
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
		},
	)

	// Claim
	g.Get(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			claim := c.Params("claim")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
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
		},
	)
	withCacheWipe.Put(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			claim := c.Params("claim")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
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
		},
	)
	withCacheWipe.Delete(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			claim := c.Params("claim")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
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
		},
	)
}

func registerSubordinateMetadataPolicies(
	r fiber.Router, subordinates model.SubordinateStorageBackend, kv model.KeyValueStore,
) {
	g := r.Group("/subordinates/:subordinateID/metadata-policies")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// Helpers shared with general metadata policies.
	getPolicy := func(mp *oidfed.MetadataPolicies, et string) oidfed.MetadataPolicy {
		switch et {
		case "openid_provider":
			return mp.OpenIDProvider
		case "openid_relying_party":
			return mp.RelyingParty
		case "oauth_authorization_server":
			return mp.OAuthAuthorizationServer
		case "oauth_client":
			return mp.OAuthClient
		case "oauth_resource":
			return mp.OAuthProtectedResource
		case "federation_entity":
			return mp.FederationEntity
		default:
			if mp.Extra != nil {
				return mp.Extra[et]
			}
		}
		return nil
	}
	setPolicy := func(mp *oidfed.MetadataPolicies, et string, policy oidfed.MetadataPolicy) {
		switch et {
		case "openid_provider":
			mp.OpenIDProvider = policy
		case "openid_relying_party":
			mp.RelyingParty = policy
		case "oauth_authorization_server":
			mp.OAuthAuthorizationServer = policy
		case "oauth_client":
			mp.OAuthClient = policy
		case "oauth_resource":
			mp.OAuthProtectedResource = policy
		case "federation_entity":
			mp.FederationEntity = policy
		default:
			if mp.Extra == nil {
				mp.Extra = map[string]oidfed.MetadataPolicy{}
			}
			mp.Extra[et] = policy
		}
	}
	setEntry := func(mp *oidfed.MetadataPolicies, et, claim string, entry oidfed.MetadataPolicyEntry) {
		policy := getPolicy(mp, et)
		if policy == nil {
			policy = oidfed.MetadataPolicy{}
		}
		policy[claim] = entry
		setPolicy(mp, et, policy)
	}

	getGeneral := func() (*oidfed.MetadataPolicies, bool, error) {
		var mp oidfed.MetadataPolicies
		found, err := kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
		if err != nil {
			return nil, false, err
		}
		if !found {
			return nil, false, nil
		}
		return &mp, true, nil
	}

	getSub := func(dbID string) (*model.ExtendedSubordinateInfo, error) {
		info, err := subordinates.GetByDBID(dbID)
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, model.NotFoundError("subordinate not found")
		}
		return info, nil
	}

	writeNotFound := func(c *fiber.Ctx, msg string) error {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(msg))
	}
	writeServerError := func(c *fiber.Ctx, err error) error {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	writeBadBody := func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}

	// GET full subordinate-specific policies
	g.Get(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			if info.MetadataPolicy == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			return c.JSON(info.MetadataPolicy)
		},
	)

	// PUT replace full subordinate-specific policies
	withCacheWipe.Put(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			var body oidfed.MetadataPolicies
			if err := c.BodyParser(&body); err != nil {
				return writeBadBody(c)
			}
			info.MetadataPolicy = &body
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			return c.JSON(body)
		},
	)

	// POST copy general metadata policies to subordinate
	withCacheWipe.Post(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			general, found, err := getGeneral()
			if err != nil {
				return writeServerError(c, err)
			}
			if !found || general == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			} else {
				copied := *general
				info.MetadataPolicy = &copied
			}
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			return c.Status(fiber.StatusCreated).JSON(info.MetadataPolicy)
		},
	)

	// DELETE subordinate-specific policies
	withCacheWipe.Delete(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			if info.MetadataPolicy == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			info.MetadataPolicy = nil
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Entity type handlers
	g.Get(
		"/:entityType", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			if info.MetadataPolicy == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			policy := getPolicy(info.MetadataPolicy, et)
			if policy == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			return c.JSON(policy)
		},
	)
	withCacheWipe.Put(
		"/:entityType", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			var body oidfed.MetadataPolicy
			if err := c.BodyParser(&body); err != nil {
				return writeBadBody(c)
			}
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			setPolicy(info.MetadataPolicy, et, body)
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			return c.JSON(body)
		},
	)
	withCacheWipe.Post(
		"/:entityType", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			var body oidfed.MetadataPolicy
			if err := c.BodyParser(&body); err != nil {
				return writeBadBody(c)
			}
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			existing := getPolicy(info.MetadataPolicy, et)
			if existing == nil {
				existing = oidfed.MetadataPolicy{}
			}
			for claim, ops := range body {
				existing[claim] = ops
			}
			setPolicy(info.MetadataPolicy, et, existing)
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			return c.JSON(existing)
		},
	)
	withCacheWipe.Delete(
		"/:entityType", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			if info.MetadataPolicy == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			if info.MetadataPolicy != nil {
				if et == "openid_provider" {
					info.MetadataPolicy.OpenIDProvider = nil
				} else if et == "openid_relying_party" {
					info.MetadataPolicy.RelyingParty = nil
				} else if et == "oauth_authorization_server" {
					info.MetadataPolicy.OAuthAuthorizationServer = nil
				} else if et == "oauth_client" {
					info.MetadataPolicy.OAuthClient = nil
				} else if et == "oauth_resource" {
					info.MetadataPolicy.OAuthProtectedResource = nil
				} else if et == "federation_entity" {
					info.MetadataPolicy.FederationEntity = nil
				} else if info.MetadataPolicy.Extra != nil {
					delete(info.MetadataPolicy.Extra, et)
				}
			}
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Claim handlers
	g.Get(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			claim := c.Params("claim")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			if info.MetadataPolicy == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			policy := getPolicy(info.MetadataPolicy, et)
			if policy == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			ops := policy[claim]
			if ops == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			return c.JSON(ops)
		},
	)
	withCacheWipe.Put(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			claim := c.Params("claim")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			var body oidfed.MetadataPolicyEntry
			if err := c.BodyParser(&body); err != nil {
				return writeBadBody(c)
			}
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			setEntry(info.MetadataPolicy, et, claim, body)
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			return c.JSON(body)
		},
	)
	withCacheWipe.Post(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			claim := c.Params("claim")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			var body oidfed.MetadataPolicyEntry
			if err := c.BodyParser(&body); err != nil {
				return writeBadBody(c)
			}
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			policy := getPolicy(info.MetadataPolicy, et)
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
			setPolicy(info.MetadataPolicy, et, policy)
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			return c.JSON(existing)
		},
	)
	withCacheWipe.Delete(
		"/:entityType/:claim", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			claim := c.Params("claim")
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			if info.MetadataPolicy == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			if info.MetadataPolicy != nil {
				policy := getPolicy(info.MetadataPolicy, et)
				if policy == nil {
					return writeNotFound(c, "metadata policy not found")
				}
				if _, ok := policy[claim]; !ok {
					return writeNotFound(c, "metadata policy not found")
				}
				delete(policy, claim)
				setPolicy(info.MetadataPolicy, et, policy)
			}
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Operator handlers
	g.Get(
		"/:entityType/:claim/:operator", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			claim := c.Params("claim")
			op := oidfed.PolicyOperatorName(c.Params("operator"))
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			if info.MetadataPolicy == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			policy := getPolicy(info.MetadataPolicy, et)
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
		},
	)
	withCacheWipe.Put(
		"/:entityType/:claim/:operator", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			claim := c.Params("claim")
			op := oidfed.PolicyOperatorName(c.Params("operator"))
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			var body any
			if err := json.Unmarshal(c.Body(), &body); err != nil {
				return writeBadBody(c)
			}

			created := false
			if info.MetadataPolicy == nil {
				info.MetadataPolicy = &oidfed.MetadataPolicies{}
			}
			policy := getPolicy(info.MetadataPolicy, et)
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
			setPolicy(info.MetadataPolicy, et, policy)
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			if created {
				return c.Status(fiber.StatusCreated).JSON(body)
			}
			return c.JSON(body)
		},
	)
	withCacheWipe.Delete(
		"/:entityType/:claim/:operator", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			et := c.Params("entityType")
			claim := c.Params("claim")
			op := oidfed.PolicyOperatorName(c.Params("operator"))
			info, err := getSub(id)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			if info.MetadataPolicy == nil {
				return writeNotFound(c, "metadata policy not found")
			}
			if info.MetadataPolicy != nil {
				policy := getPolicy(info.MetadataPolicy, et)
				if policy != nil {
					entry := policy[claim]
					if entry != nil {
						if _, ok := entry[op]; !ok {
							return writeNotFound(c, "metadata policy not found")
						}
						delete(entry, op)
						policy[claim] = entry
						if len(entry) == 0 {
							delete(policy, claim)
						}
						setPolicy(info.MetadataPolicy, et, policy)
					}
				}
			}
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return writeServerError(c, err)
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
}

func registerSubordinateConstraints(
	r fiber.Router, subordinates model.SubordinateStorageBackend, kv model.KeyValueStore,
) {
	g := r.Group("/subordinates/:subordinateID/constraints")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// Helper to get general constraints from KV store
	getGeneralConstraints := func() (*oidfed.ConstraintSpecification, bool, error) {
		var cs oidfed.ConstraintSpecification
		found, err := kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &cs)
		if err != nil {
			return nil, false, err
		}
		if !found {
			return nil, false, nil
		}
		return &cs, true, nil
	}

	// Subordinate-specific constraints (stored per subordinate row)
	g.Get(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			if info.Constraints == nil {
				return c.JSON(oidfed.ConstraintSpecification{})
			}
			return c.JSON(info.Constraints)
		},
	)
	withCacheWipe.Put(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			var body oidfed.ConstraintSpecification
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if body.MaxPathLength != nil && *body.MaxPathLength < 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("max_path_length must be >= 0"))
			}
			info.Constraints = &body
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(body)
		},
	)
	// POST /: Copy general constraints to this subordinate
	withCacheWipe.Post(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			general, found, err := getGeneralConstraints()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found || general == nil {
				// No general constraints set, copy empty
				info.Constraints = &oidfed.ConstraintSpecification{}
			} else {
				// Deep copy general constraints
				copied := *general
				info.Constraints = &copied
			}
			if err = subordinates.Update(info.EntityID, *info); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(info.Constraints)
		},
	)
	withCacheWipe.Delete(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			info.Constraints = nil
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Max path length
	g.Get(
		"/max-path-length", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			if info.Constraints == nil || info.Constraints.MaxPathLength == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("max_path_length not set"))
			}
			return c.JSON(*info.Constraints.MaxPathLength)
		},
	)
	withCacheWipe.Put(
		"/max-path-length", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			if len(c.Body()) == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("empty body"))
			}
			var mpl int
			if err := json.Unmarshal(c.Body(), &mpl); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if mpl < 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("max_path_length must be >= 0"))
			}
			if info.Constraints == nil {
				info.Constraints = &oidfed.ConstraintSpecification{}
			}
			info.Constraints.MaxPathLength = &mpl
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(mpl)
		},
	)
	withCacheWipe.Delete(
		"/max-path-length", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			if info.Constraints != nil {
				info.Constraints.MaxPathLength = nil
				if err := subordinates.Update(info.EntityID, *info); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Naming constraints
	g.Get(
		"/naming-constraints", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			if info.Constraints == nil || info.Constraints.NamingConstraints == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("naming_constraints not set"))
			}
			return c.JSON(info.Constraints.NamingConstraints)
		},
	)
	withCacheWipe.Put(
		"/naming-constraints", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			var body oidfed.NamingConstraints
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if info.Constraints == nil {
				info.Constraints = &oidfed.ConstraintSpecification{}
			}
			info.Constraints.NamingConstraints = &body
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(body)
		},
	)
	withCacheWipe.Delete(
		"/naming-constraints", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			if info.Constraints != nil {
				info.Constraints.NamingConstraints = nil
				if err := subordinates.Update(info.EntityID, *info); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Allowed entity types
	g.Get(
		"/allowed-entity-types", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			if info.Constraints == nil || info.Constraints.AllowedEntityTypes == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("allowed_entity_types not set"))
			}
			return c.JSON(info.Constraints.AllowedEntityTypes)
		},
	)
	withCacheWipe.Put(
		"/allowed-entity-types", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			var body []string
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if info.Constraints == nil {
				info.Constraints = &oidfed.ConstraintSpecification{}
			}
			info.Constraints.AllowedEntityTypes = body
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(info.Constraints.AllowedEntityTypes)
		},
	)
	withCacheWipe.Post(
		"/allowed-entity-types", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			if len(c.Body()) == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("empty body"))
			}
			entityType := string(c.Body())
			if info.Constraints == nil {
				info.Constraints = &oidfed.ConstraintSpecification{}
			}
			// Check if already exists
			for _, t := range info.Constraints.AllowedEntityTypes {
				if t == entityType {
					return c.Status(fiber.StatusCreated).JSON(info.Constraints.AllowedEntityTypes)
				}
			}
			info.Constraints.AllowedEntityTypes = append(info.Constraints.AllowedEntityTypes, entityType)
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(info.Constraints.AllowedEntityTypes)
		},
	)
	withCacheWipe.Delete(
		"/allowed-entity-types/:entityType", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			entityType := c.Params("entityType")
			info, err := subordinates.GetByDBID(id)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if info == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("subordinate not found"))
			}
			if info.Constraints == nil {
				return c.SendStatus(fiber.StatusNoContent)
			}
			updated := make([]string, 0, len(info.Constraints.AllowedEntityTypes))
			removed := false
			for _, t := range info.Constraints.AllowedEntityTypes {
				if t == entityType {
					removed = true
					continue
				}
				updated = append(updated, t)
			}
			if !removed {
				return c.SendStatus(fiber.StatusNoContent)
			}
			info.Constraints.AllowedEntityTypes = updated
			if err := subordinates.Update(info.EntityID, *info); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(info.Constraints.AllowedEntityTypes)
		},
	)
}

func registerGeneralConstraints(
	r fiber.Router, kv model.KeyValueStore,
) {
	g := r.Group("/subordinates/constraints")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	getGeneral := func() (*oidfed.ConstraintSpecification, bool, error) {
		var cs oidfed.ConstraintSpecification
		found, err := kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &cs)
		if err != nil {
			return nil, false, err
		}
		if !found {
			return nil, false, nil
		}
		return &cs, true, nil
	}
	setGeneral := func(cs *oidfed.ConstraintSpecification) error {
		if cs == nil {
			return kv.Delete(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints)
		}
		return kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, *cs)
	}

	// General constraints
	g.Get(
		"/", func(c *fiber.Ctx) error {
			cs, found, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("constraints not set"))
			}
			return c.JSON(cs)
		},
	)
	withCacheWipe.Put(
		"/", func(c *fiber.Ctx) error {
			var body oidfed.ConstraintSpecification
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if body.MaxPathLength != nil && *body.MaxPathLength < 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("max_path_length must be >= 0"))
			}
			if err := setGeneral(&body); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(body)
		},
	)

	// Allowed entity types
	g.Get(
		"/allowed-entity-types", func(c *fiber.Ctx) error {
			cs, found, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("allowed_entity_types not set"))
			}
			if cs.AllowedEntityTypes == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("allowed_entity_types not set"))
			}
			return c.JSON(cs.AllowedEntityTypes)
		},
	)
	withCacheWipe.Put(
		"/allowed-entity-types", func(c *fiber.Ctx) error {
			var body []string
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			cs, _, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if cs == nil {
				cs = &oidfed.ConstraintSpecification{}
			}
			cs.AllowedEntityTypes = body
			if err := setGeneral(cs); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(cs.AllowedEntityTypes)
		},
	)
	withCacheWipe.Post(
		"/allowed-entity-types", func(c *fiber.Ctx) error {
			if len(c.Body()) == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("empty body"))
			}
			entityType := string(c.Body())
			cs, _, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if cs == nil {
				cs = &oidfed.ConstraintSpecification{}
			}
			for _, t := range cs.AllowedEntityTypes {
				if t == entityType {
					return c.Status(fiber.StatusCreated).JSON(cs.AllowedEntityTypes)
				}
			}
			cs.AllowedEntityTypes = append(cs.AllowedEntityTypes, entityType)
			if err := setGeneral(cs); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(cs.AllowedEntityTypes)
		},
	)
	withCacheWipe.Delete(
		"/allowed-entity-types/:entityType", func(c *fiber.Ctx) error {
			entityType := c.Params("entityType")
			cs, found, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found {
				return c.SendStatus(fiber.StatusNoContent)
			}
			updated := make([]string, 0, len(cs.AllowedEntityTypes))
			removed := false
			for _, t := range cs.AllowedEntityTypes {
				if t == entityType {
					removed = true
					continue
				}
				updated = append(updated, t)
			}
			if !removed {
				return c.SendStatus(fiber.StatusNoContent)
			}
			cs.AllowedEntityTypes = updated
			if err := setGeneral(cs); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(cs.AllowedEntityTypes)
		},
	)

	// Max path length
	g.Get(
		"/max-path-length", func(c *fiber.Ctx) error {
			cs, found, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found || cs.MaxPathLength == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("max_path_length not set"))
			}
			return c.JSON(*cs.MaxPathLength)
		},
	)
	withCacheWipe.Put(
		"/max-path-length", func(c *fiber.Ctx) error {
			if len(c.Body()) == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("empty body"))
			}
			var mpl int
			if err := json.Unmarshal(c.Body(), &mpl); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if mpl < 0 {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("max_path_length must be >= 0"))
			}
			cs, _, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if cs == nil {
				cs = &oidfed.ConstraintSpecification{}
			}
			cs.MaxPathLength = &mpl
			if err := setGeneral(cs); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(mpl)
		},
	)
	withCacheWipe.Delete(
		"/max-path-length", func(c *fiber.Ctx) error {
			cs, found, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found {
				return c.SendStatus(fiber.StatusNoContent)
			}
			cs.MaxPathLength = nil
			if err := setGeneral(cs); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Naming constraints
	g.Get(
		"/naming-constraints", func(c *fiber.Ctx) error {
			cs, found, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found || cs.NamingConstraints == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("naming_constraints not set"))
			}
			return c.JSON(cs.NamingConstraints)
		},
	)
	withCacheWipe.Put(
		"/naming-constraints", func(c *fiber.Ctx) error {
			var body oidfed.NamingConstraints
			if err := c.BodyParser(&body); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			cs, _, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if cs == nil {
				cs = &oidfed.ConstraintSpecification{}
			}
			cs.NamingConstraints = &body
			if err := setGeneral(cs); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(body)
		},
	)
	withCacheWipe.Delete(
		"/naming-constraints", func(c *fiber.Ctx) error {
			cs, found, err := getGeneral()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found {
				return c.SendStatus(fiber.StatusNoContent)
			}
			cs.NamingConstraints = nil
			if err := setGeneral(cs); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

}

func registerSubordinateKeys(r fiber.Router, subordinates model.SubordinateStorageBackend) {
	g := r.Group("/subordinates/:subordinateID/jwks")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	getSub := func(dbID string) (*model.ExtendedSubordinateInfo, error) {
		info, err := subordinates.GetByDBID(dbID)
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, model.NotFoundError("subordinate not found")
		}
		return info, nil
	}
	writeNotFound := func(c *fiber.Ctx, msg string) error {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(msg))
	}
	writeServerError := func(c *fiber.Ctx, err error) error {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	writeBadBody := func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}

	// GET subordinate JWKS
	g.Get(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			// Return empty JWKS if none exists
			if info.JWKS.Keys.Set == nil {
				return c.JSON(fiber.Map{"keys": []any{}})
			}
			return c.JSON(info.JWKS)
		},
	)

	// PUT replace subordinate JWKS
	withCacheWipe.Put(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			// Verify subordinate exists
			if _, err := getSub(id); err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
			}
			var body model.JWKS
			if err := c.BodyParser(&body); err != nil {
				return writeBadBody(c)
			}
			updatedJWKS, err := subordinates.UpdateJWKSByDBID(id, body)
			if err != nil {
				return writeServerError(c, err)
			}
			return c.JSON(updatedJWKS)
		},
	)

	// POST add JWK to subordinate JWKS
	withCacheWipe.Post(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
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
			return c.Status(fiber.StatusCreated).JSON(updatedJWKS)
		},
	)

	// DELETE remove JWK by kid from subordinate JWKS
	withCacheWipe.Delete(
		"/:kid", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			kid := c.Params("kid")
			info, err := getSub(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return writeNotFound(c, err.Error())
				}
				return writeServerError(c, err)
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
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
}

// Subordinate crit is managed via the additional-claims endpoints; no separate crit endpoints

func registerSubordinateMetadataPolicyCrit(r fiber.Router) {
	g := r.Group("/subordinates/metadata-policy-crit")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	withCacheWipe.Put("/", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	withCacheWipe.Post("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })
	withCacheWipe.Delete("/:operator", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}

// registerGeneralSubordinateLifetime adds handlers for the general subordinate lifetime endpoints.
func registerGeneralSubordinateLifetime(r fiber.Router, kv model.KeyValueStore) {
	g := r.Group("/subordinates")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET /subordinates/lifetime - Get general subordinate lifetime in seconds
	g.Get(
		"/lifetime", func(c *fiber.Ctx) error {
			var seconds int
			found, err := kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyLifetime, &seconds)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if !found || seconds <= 0 {
				seconds = int(storage.DefaultSubordinateStatementLifetime.Seconds())
			}
			return c.JSON(seconds)
		},
	)

	// PUT /subordinates/lifetime - Update general subordinate lifetime in seconds
	withCacheWipe.Put(
		"/lifetime", func(c *fiber.Ctx) error {
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
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyLifetime, seconds,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(seconds)
		},
	)
}

// registerSubordinateAdditionalClaims adds handlers for subordinate-specific additional claims.
func registerSubordinateAdditionalClaims(r fiber.Router, subordinates model.SubordinateStorageBackend) {
	g := r.Group("/subordinates/:subordinateID/additional-claims")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - List all additional claims for a subordinate
	g.Get(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			claims, err := subordinates.ListAdditionalClaims(id)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(err.Error()))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(claims)
		},
	)

	// PUT / - Replace all additional claims for a subordinate
	withCacheWipe.Put(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			var req []model.AddAdditionalClaim
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			claims, err := subordinates.SetAdditionalClaims(id, req)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(err.Error()))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(claims)
		},
	)

	// POST / - Create a single additional claim
	withCacheWipe.Post(
		"/", func(c *fiber.Ctx) error {
			id := c.Params("subordinateID")
			var req model.AddAdditionalClaim
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			claim, err := subordinates.CreateAdditionalClaim(id, req)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(err.Error()))
				}
				var ae model.AlreadyExistsError
				if errors.As(err, &ae) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(err.Error()))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(claim)
		},
	)

	// GET /:additionalClaimsID - Get a single additional claim
	g.Get(
		"/:additionalClaimsID", func(c *fiber.Ctx) error {
			subID := c.Params("subordinateID")
			claimID := c.Params("additionalClaimsID")
			claim, err := subordinates.GetAdditionalClaim(subID, claimID)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(err.Error()))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(claim)
		},
	)

	// PUT /:additionalClaimsID - Update a single additional claim
	withCacheWipe.Put(
		"/:additionalClaimsID", func(c *fiber.Ctx) error {
			subID := c.Params("subordinateID")
			claimID := c.Params("additionalClaimsID")
			var req model.AddAdditionalClaim
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			claim, err := subordinates.UpdateAdditionalClaim(subID, claimID, req)
			if err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(err.Error()))
				}
				var ae model.AlreadyExistsError
				if errors.As(err, &ae) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(err.Error()))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(claim)
		},
	)

	// DELETE /:additionalClaimsID - Delete a single additional claim
	withCacheWipe.Delete(
		"/:additionalClaimsID", func(c *fiber.Ctx) error {
			subID := c.Params("subordinateID")
			claimID := c.Params("additionalClaimsID")
			if err := subordinates.DeleteAdditionalClaim(subID, claimID); err != nil {
				var nf model.NotFoundError
				if errors.As(err, &nf) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(err.Error()))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
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

	// Helper to load claims from KV
	loadClaims := func() ([]generalAdditionalClaim, error) {
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

	// Helper to save claims to KV
	saveClaims := func(claims []generalAdditionalClaim) error {
		return kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, claims)
	}

	// Helper to find next ID
	nextID := func(claims []generalAdditionalClaim) int {
		maxID := 0
		for _, c := range claims {
			if c.ID > maxID {
				maxID = c.ID
			}
		}
		return maxID + 1
	}

	// GET / - List all general additional claims
	g.Get(
		"/", func(c *fiber.Ctx) error {
			claims, err := loadClaims()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(claims)
		},
	)

	// PUT / - Replace all general additional claims
	withCacheWipe.Put(
		"/", func(c *fiber.Ctx) error {
			var req []model.AddAdditionalClaim
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
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
			if err := saveClaims(claims); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(claims)
		},
	)

	// POST / - Add a single general additional claim
	withCacheWipe.Post(
		"/", func(c *fiber.Ctx) error {
			var req model.AddAdditionalClaim
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			claims, err := loadClaims()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			// Check for duplicate claim name
			for _, existing := range claims {
				if existing.Claim == req.Claim {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("claim already exists"))
				}
			}
			newClaim := generalAdditionalClaim{
				ID:    nextID(claims),
				Claim: req.Claim,
				Value: req.Value,
				Crit:  req.Crit,
			}
			claims = append(claims, newClaim)
			if err = saveClaims(claims); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(newClaim)
		},
	)

	// GET /:additionalClaimsID - Get a single general additional claim
	g.Get(
		"/:additionalClaimsID", func(c *fiber.Ctx) error {
			idStr := c.Params("additionalClaimsID")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid additionalClaimsID"))
			}
			claims, err := loadClaims()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			for _, claim := range claims {
				if claim.ID == id {
					return c.JSON(claim)
				}
			}
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("additional claim not found"))
		},
	)

	// PUT /:additionalClaimsID - Update a single general additional claim
	withCacheWipe.Put(
		"/:additionalClaimsID", func(c *fiber.Ctx) error {
			idStr := c.Params("additionalClaimsID")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid additionalClaimsID"))
			}
			var req model.AddAdditionalClaim
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			claims, err := loadClaims()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			found := -1
			for i, claim := range claims {
				if claim.ID == id {
					found = i
					break
				}
			}
			if found == -1 {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("additional claim not found"))
			}
			// Check for duplicate claim name (excluding current)
			if req.Claim != "" && req.Claim != claims[found].Claim {
				for i, existing := range claims {
					if i != found && existing.Claim == req.Claim {
						return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("claim already exists"))
					}
				}
				claims[found].Claim = req.Claim
			}
			claims[found].Value = req.Value
			claims[found].Crit = req.Crit
			if err := saveClaims(claims); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(claims[found])
		},
	)

	// DELETE /:additionalClaimsID - Delete a single general additional claim
	withCacheWipe.Delete(
		"/:additionalClaimsID", func(c *fiber.Ctx) error {
			idStr := c.Params("additionalClaimsID")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid additionalClaimsID"))
			}
			claims, err := loadClaims()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			found := -1
			for i, claim := range claims {
				if claim.ID == id {
					found = i
					break
				}
			}
			if found == -1 {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("additional claim not found"))
			}
			claims = append(claims[:found], claims[found+1:]...)
			if err := saveClaims(claims); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
}
