// Package adminapi provides the admin API for managing subordinates in the lighthouse federation.
//
// This file serves as the entry point for subordinate-related handlers.
// The actual handler implementations are split across multiple files:
//   - subordinates_base.go: Basic CRUD operations (list, create, get, update, delete)
//   - subordinates_metadata.go: Subordinate metadata endpoints
//   - subordinates_metadata_policies.go: Metadata policy endpoints (general and subordinate-specific)
//   - subordinates_constraints.go: Constraint endpoints
//   - subordinates_keys.go: JWKS endpoints
//   - subordinates_additional_claims.go: Additional claims endpoints
//   - subordinates_statement.go: Statement preview endpoint
//   - subordinates_lifetime.go: Lifetime configuration endpoint
//   - subordinates_helpers.go: Shared helper functions
package adminapi

import (
	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// DefaultSubordinateStatus is the default status for newly created subordinates.
var DefaultSubordinateStatus = model.StatusActive

// RegisterSubordinateHandlers registers all subordinate-related handlers on the given router.
// This includes basic CRUD, metadata, metadata policies, constraints, keys, additional claims,
// statement preview, and lifetime configuration endpoints.
//
// All write operations are wrapped in database transactions to ensure atomicity of
// data changes and event recording.
func RegisterSubordinateHandlers(
	r fiber.Router,
	storages model.Backends,
	fedEntity oidfed.FederationEntity,
) {
	// Register general endpoints first (routes without :subordinateID in the path)
	// These must be registered before subordinate-specific routes to avoid conflicts

	// General metadata policies: /subordinates/metadata-policies/*
	registerGeneralMetadataPolicies(r, storages.KV)

	// General metadata policy crit: /subordinates/metadata-policy-crit/*
	registerSubordinateMetadataPolicyCrit(r, storages.KV)

	// General constraints: /subordinates/constraints/*
	registerGeneralConstraints(r, storages.KV)

	// General additional claims: /subordinates/additional-claims/*
	registerGeneralAdditionalClaims(r, storages.KV)

	// General lifetime: /subordinates/lifetime
	registerGeneralSubordinateLifetime(r, storages.KV)

	// Base CRUD operations: /subordinates, /subordinates/:subordinateID, etc.
	registerSubordinatesBase(r, storages)

	// Statement preview: /subordinates/:subordinateID/statement
	registerSubordinateStatement(r, storages.Subordinates, storages.KV, fedEntity)

	// Subordinate-specific metadata: /subordinates/:subordinateID/metadata/*
	registerSubordinateMetadata(r, storages)

	// Subordinate-specific metadata policies: /subordinates/:subordinateID/metadata-policies/*
	registerSubordinateMetadataPolicies(r, storages)

	// Subordinate-specific constraints: /subordinates/:subordinateID/constraints/*
	registerSubordinateConstraints(r, storages)

	// Subordinate-specific JWKS: /subordinates/:subordinateID/jwks/*
	registerSubordinateKeys(r, storages)

	// Subordinate-specific additional claims: /subordinates/:subordinateID/additional-claims/*
	registerSubordinateAdditionalClaims(r, storages)
}
