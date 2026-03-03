package adminapi

import (
	"errors"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// Common HTTP error response helpers

// writeNotFound returns a 404 JSON error response.
func writeNotFound(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(msg))
}

// writeServerError returns a 500 JSON error response.
func writeServerError(c *fiber.Ctx, err error) error {
	return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
}

// writeBadBody returns a 400 JSON error response for invalid request bodies.
func writeBadBody(c *fiber.Ctx) error {
	return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
}

// writeBadRequest returns a 400 JSON error response with a custom message.
func writeBadRequest(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(msg))
}

// writeConflict returns a 409 JSON error response.
func writeConflict(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(msg))
}

// Subordinate lookup helpers

// getSubordinateByDBID retrieves a subordinate by database ID, returning an error if not found.
func getSubordinateByDBID(subordinates model.SubordinateStorageBackend, dbID string) (*model.ExtendedSubordinateInfo, error) {
	info, err := subordinates.GetByDBID(dbID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, model.NotFoundError("subordinate not found")
	}
	return info, nil
}

// handleSubordinateLookup retrieves a subordinate and handles error responses.
// Returns nil if an error response was written.
func handleSubordinateLookup(c *fiber.Ctx, subordinates model.SubordinateStorageBackend) (*model.ExtendedSubordinateInfo, error) {
	id := c.Params("subordinateID")
	info, err := getSubordinateByDBID(subordinates, id)
	if err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return nil, writeNotFound(c, err.Error())
		}
		return nil, writeServerError(c, err)
	}
	return info, nil
}

// Metadata policy helpers

// getMetadataPolicy retrieves a metadata policy for a given entity type.
func getMetadataPolicy(mp *oidfed.MetadataPolicies, et string) oidfed.MetadataPolicy {
	if mp == nil {
		return nil
	}
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

// setMetadataPolicy sets a metadata policy for a given entity type.
func setMetadataPolicy(mp *oidfed.MetadataPolicies, et string, policy oidfed.MetadataPolicy) {
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

// deleteMetadataPolicy removes a metadata policy for a given entity type.
func deleteMetadataPolicy(mp *oidfed.MetadataPolicies, et string) {
	if mp == nil {
		return
	}
	switch et {
	case "openid_provider":
		mp.OpenIDProvider = nil
	case "openid_relying_party":
		mp.RelyingParty = nil
	case "oauth_authorization_server":
		mp.OAuthAuthorizationServer = nil
	case "oauth_client":
		mp.OAuthClient = nil
	case "oauth_resource":
		mp.OAuthProtectedResource = nil
	case "federation_entity":
		mp.FederationEntity = nil
	default:
		if mp.Extra != nil {
			delete(mp.Extra, et)
		}
	}
}

// getMetadataPolicyEntry retrieves a metadata policy entry for a given entity type and claim.
func getMetadataPolicyEntry(mp *oidfed.MetadataPolicies, et, claim string) oidfed.MetadataPolicyEntry {
	policy := getMetadataPolicy(mp, et)
	if policy == nil {
		return nil
	}
	return policy[claim]
}

// setMetadataPolicyEntry sets a metadata policy entry for a given entity type and claim.
func setMetadataPolicyEntry(mp *oidfed.MetadataPolicies, et, claim string, entry oidfed.MetadataPolicyEntry) {
	policy := getMetadataPolicy(mp, et)
	if policy == nil {
		policy = oidfed.MetadataPolicy{}
	}
	policy[claim] = entry
	setMetadataPolicy(mp, et, policy)
}

// Metadata helpers

// getEntityMetadata retrieves entity-specific metadata from Extra field.
func getEntityMetadata(md *oidfed.Metadata, et string) map[string]any {
	if md == nil || md.Extra == nil {
		return nil
	}
	v, ok := md.Extra[et]
	if !ok {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m
}

// setEntityMetadata sets entity-specific metadata in Extra field.
func setEntityMetadata(md *oidfed.Metadata, et string, m map[string]any) {
	if md.Extra == nil {
		md.Extra = map[string]any{}
	}
	md.Extra[et] = m
}

// deleteEntityMetadata removes entity-specific metadata from Extra field.
func deleteEntityMetadata(md *oidfed.Metadata, et string) {
	if md == nil || md.Extra == nil {
		return
	}
	delete(md.Extra, et)
}

// subordinateHasKeys checks if a subordinate has any JWKS keys defined.
func subordinateHasKeys(info *model.ExtendedSubordinateInfo) bool {
	if info == nil {
		return false
	}
	return info.JWKS.Keys.Set != nil && info.JWKS.Keys.Len() > 0
}

// jwksHasKeys checks if a JWKS has any keys defined.
func jwksHasKeys(jwks *model.JWKS) bool {
	if jwks == nil {
		return false
	}
	return jwks.Keys.Set != nil && jwks.Keys.Len() > 0
}
