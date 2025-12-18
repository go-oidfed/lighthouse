package adminapi

import (
	"fmt"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

func registerSubordinates(r fiber.Router) {
	g := r.Group("/subordinates")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON([]fiber.Map{}) })
	g.Post("/", func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) })
	g.Get("/:subordinateID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:subordinateID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:subordinateID", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:subordinateID/statement", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"statement": fiber.Map{}}) })
	g.Get("/:subordinateID/history", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"events": []fiber.Map{}}) })

	// Subordinate additional claims
	g.Get("/:subordinateID/additional-claims", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:subordinateID/additional-claims", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Get(
		"/:subordinateID/additional-claims/:additionalClaimsID",
		func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) },
	)
	withCacheWipe.Put(
		"/:subordinateID/additional-claims/:additionalClaimsID",
		func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) },
	)
	withCacheWipe.Delete(
		"/:subordinateID/additional-claims/:additionalClaimsID",
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)
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
	g.Get(
		"/ping", func(c *fiber.Ctx) error {
			fmt.Println("PING")
			return c.SendString("ok")
		},
	) // TODO remove
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
			if err := storagesKV.SetAny(
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
			if err := storagesKV.SetAny(
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
			op := c.Params("operator")
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
			val, ok := entry[oidfed.PolicyOperatorName(op)]
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
			op := c.Params("operator")
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
			} else if _, ok := entry[oidfed.PolicyOperatorName(op)]; !ok {
				created = 1
			}
			entry[oidfed.PolicyOperatorName(op)] = val
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
			op := c.Params("operator")
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
			} else if _, ok := entry[oidfed.PolicyOperatorName(op)]; !ok {
				created = 1
			}
			entry[oidfed.PolicyOperatorName(op)] = val
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
			op := c.Params("operator")
			var mp map[string]map[string]map[string]any
			_, _ = storagesKV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &mp)
			if mp != nil {
				if m := mp[et]; m != nil {
					if ops := m[claim]; ops != nil {
						delete(ops, op)
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

func registerSubordinateMetadata(r fiber.Router) {
	g := r.Group("/subordinates/:subordinateID/metadata")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Get("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Post("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:entityType", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:entityType/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}

func registerSubordinateMetadataPolicies(r fiber.Router) {
	g := r.Group("/subordinates/:subordinateID/metadata-policies")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Post("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Post("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:entityType", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:entityType/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}

func registerSubordinateConstraints(r fiber.Router) {
	withCacheWipe := r.Use(subordinateStatementsCacheInvalidationMiddleware)
	// General constraints
	r.Get("/subordinates/constraints", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/subordinates/constraints", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })

	r.Get("/subordinates/:subordinateID/constraints", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put(
		"/subordinates/:subordinateID/constraints", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) },
	)
	withCacheWipe.Post(
		"/subordinates/:subordinateID/constraints",
		func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) },
	)
	withCacheWipe.Delete(
		"/subordinates/:subordinateID/constraints",
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)

	// Allowed entity types
	r.Get("/subordinates/constraints/allowed-entity-types", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	withCacheWipe.Put(
		"/subordinates/constraints/allowed-entity-types", func(c *fiber.Ctx) error { return c.JSON([]string{}) },
	)
	withCacheWipe.Post(
		"/subordinates/constraints/allowed-entity-types",
		func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON([]string{}) },
	)
	withCacheWipe.Delete(
		"/subordinates/constraints/allowed-entity-types/:entityType",
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)

	// Max path length
	r.Get("/subordinates/constraints/max-path-length", func(c *fiber.Ctx) error { return c.JSON(nil) })
	withCacheWipe.Put("/subordinates/constraints/max-path-length", func(c *fiber.Ctx) error { return c.JSON(0) })
}

func registerSubordinateKeys(r fiber.Router) {
	g := r.Group("/subordinates/:subordinateID/jwks")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"jwks": []fiber.Map{}}) })
	withCacheWipe.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"jwks": []fiber.Map{}}) })
	withCacheWipe.Post(
		"/", func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{"jwks": []fiber.Map{}}) },
	)
	withCacheWipe.Delete("/:kid", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
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
