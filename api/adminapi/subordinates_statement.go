package adminapi

import (
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerSubordinateStatement adds the handler for previewing subordinate statement data.
func registerSubordinateStatement(
	r fiber.Router,
	subordinates model.SubordinateStorageBackend,
	kv model.KeyValueStore,
	fedEntity oidfed.FederationEntity,
) {
	r.Get("/subordinates/:subordinateID/statement", handleGetSubordinateStatement(subordinates, kv, fedEntity))
}

func handleGetSubordinateStatement(
	subordinates model.SubordinateStorageBackend,
	kv model.KeyValueStore,
	fedEntity oidfed.FederationEntity,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, ok := handleSubordinateLookup(c, subordinates)
		if !ok {
			return nil
		}
		// Build the statement payload
		payload := buildSubordinateStatementPayload(info, kv, fedEntity.EntityID())
		return c.JSON(payload)
	}
}

// buildSubordinateStatementPayload creates an EntityStatementPayload for preview purposes.
// This mirrors the logic in lighthouse.CreateSubordinateStatement but without SourceEndpoint
// since that's only known at the fetch endpoint.
func buildSubordinateStatementPayload(
	subordinate *model.ExtendedSubordinateInfo,
	kv model.KeyValueStore,
	issuer string,
) oidfed.EntityStatementPayload {
	now := time.Now()
	lifetime, err := storage.GetSubordinateStatementLifetime(kv)
	if err != nil {
		lifetime = storage.DefaultSubordinateStatementLifetime
	}

	// Build extra claims and critical extensions from subordinate additional claims
	extra := make(map[string]any)
	var criticalExtensions []string
	for _, claim := range subordinate.SubordinateAdditionalClaims {
		extra[claim.Claim] = claim.Value
		if claim.Crit {
			criticalExtensions = append(criticalExtensions, claim.Claim)
		}
	}

	// Load metadata policy crit from KV store and filter to only used operators
	var configuredCritOperators []oidfed.PolicyOperatorName
	_, _ = kv.GetAs(
		model.KeyValueScopeSubordinateStatement,
		model.KeyValueKeyMetadataPolicyCrit,
		&configuredCritOperators,
	)
	metadataPolicyCrit := filterUsedPolicyOperators(subordinate.MetadataPolicy, configuredCritOperators)

	return oidfed.EntityStatementPayload{
		Issuer:             issuer,
		Subject:            subordinate.EntityID,
		IssuedAt:           unixtime.Unixtime{Time: now},
		ExpiresAt:          unixtime.Unixtime{Time: now.Add(lifetime)},
		JWKS:               subordinate.JWKS.Keys,
		Metadata:           subordinate.Metadata,
		MetadataPolicy:     subordinate.MetadataPolicy,
		Constraints:        subordinate.Constraints,
		CriticalExtensions: criticalExtensions,
		MetadataPolicyCrit: metadataPolicyCrit,
		Extra:              extra,
	}
}
