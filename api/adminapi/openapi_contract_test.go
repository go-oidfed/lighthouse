package adminapi

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestOpenAPIUsesFullReplacementPUTSchema locks the PUT/PATCH contract:
//
//   - PUT request bodies reference dedicated full-replacement schemas (never
//     the shared create schemas) and require their identity field
//     (entity_id / type / issuer / trust_mark_type). Identity is never derived
//     from the path; sending the same identity keeps it, a new value renames
//     (409 on collision).
//   - Claim-row PUTs use UpdateAdditionalClaim, which requires the `claim`
//     identity field (rename supported, 409 on collision).
//   - authority-hints and subordinates expose PATCH partial-update operations
//     distinct from their full-replacement PUT.
func TestOpenAPIUsesFullReplacementPUTSchema(t *testing.T) {
	raw, err := assets.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("failed to read openapi.yaml: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("failed to parse openapi.yaml: %v", err)
	}
	paths, _ := doc["paths"].(map[string]any)
	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)

	// PUT route → required schema + identity field ("" = no identity in body).
	putIdentities := map[string]struct {
		schema   string
		identity string
	}{
		"/api/v1/admin/trust-anchors/{id}":                                                        {"UpdateTrustAnchor", "entity_id"},
		"/api/v1/admin/federation-endpoints/{type}":                                               {"UpdateFederationEndpoint", "type"},
		"/api/v1/admin/entity-configuration/authority-hints/{authorityHintID}":                    {"UpdateAuthorityHint", "entity_id"},
		"/api/v1/admin/trust-marks/types/{trustMarkTypeID}":                                       {"UpdateTrustMarkType", "trust_mark_type"},
		"/api/v1/admin/trust-marks/types/{trustMarkTypeID}/owner":                                 {"UpdateTrustMarkOwner", "entity_id"},
		"/api/v1/admin/trust-marks/owners/{ownerID}":                                              {"UpdateTrustMarkOwner", "entity_id"},
		"/api/v1/admin/trust-marks/issuers/{issuerID}":                                            {"UpdateTrustMarkIssuer", "issuer"},
		"/api/v1/admin/trust-marks/issuance-spec/{trustMarkSpecID}":                               {"UpdateTrustMarkSpec", "trust_mark_type"},
		"/api/v1/admin/trust-marks/issuance-spec/{trustMarkSpecID}/subjects/{trustMarkSubjectID}": {"UpdateTrustMarkSubject", "entity_id"},
		"/api/v1/admin/subordinates/{subordinateID}":                                              {"ReplaceSubordinate", ""},
	}

	for p, want := range putIdentities {
		got := requestSchema(pathOp(paths, p, "put"))
		if got != want.schema {
			t.Errorf("%s PUT requestBody = %q, want %s", p, got, want.schema)
			continue
		}
		if want.identity == "" {
			continue
		}
		scheme, _ := schemas[want.schema].(map[string]any)
		required, _ := scheme["required"].([]any)
		found := false
		for _, r := range required {
			if r == want.identity {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s PUT schema %s must require identity field %q", p, want.schema, want.identity)
		}
	}

	// Claim-row PUTs must use UpdateAdditionalClaim, whose `claim` identity
	// field is required (rename supported, 409 on collision).
	claimRowPuts := []string{
		"/api/v1/admin/entity-configuration/additional-claims/{additionalClaimsID}",
		"/api/v1/admin/subordinates/additional-claims/{additionalClaimsID}",
		"/api/v1/admin/subordinates/{subordinateID}/additional-claims/{additionalClaimsID}",
	}
	for _, p := range claimRowPuts {
		if got := requestSchema(pathOp(paths, p, "put")); got != "UpdateAdditionalClaim" {
			t.Errorf("%s PUT requestBody = %q, want UpdateAdditionalClaim", p, got)
		}
	}
	addClaim, _ := schemas["UpdateAdditionalClaim"].(map[string]any)
	required, _ := addClaim["required"].([]any)
	claimRequired := false
	for _, r := range required {
		if r == "claim" {
			claimRequired = true
			break
		}
	}
	if !claimRequired {
		t.Error("UpdateAdditionalClaim must require the claim identity field")
	}

	// authority-hints: PUT = full replacement, PATCH = partial (no identity).
	const ahPath = "/api/v1/admin/entity-configuration/authority-hints/{authorityHintID}"
	if got := requestSchema(pathOp(paths, ahPath, "put")); got != "UpdateAuthorityHint" {
		t.Errorf("authority hints PUT requestBody = %q, want UpdateAuthorityHint", got)
	}
	if got := requestSchema(pathOp(paths, ahPath, "patch")); got != "PatchAuthorityHint" {
		t.Errorf("authority hints PATCH requestBody = %q, want PatchAuthorityHint", got)
	}

	// subordinates: PUT = full replacement, PATCH = partial.
	const subPath = "/api/v1/admin/subordinates/{subordinateID}"
	if got := requestSchema(pathOp(paths, subPath, "patch")); got != "UpdateSubordinate" {
		t.Errorf("subordinates PATCH requestBody = %q, want UpdateSubordinate", got)
	}
}

// pathOp returns the operation map for path+method, or nil.
func pathOp(paths map[string]any, path, method string) map[string]any {
	p, _ := paths[path].(map[string]any)
	op, _ := p[method].(map[string]any)
	return op
}

// requestSchema returns the first $ref schema name in the operation's
// requestBody, or "" when absent.
func requestSchema(op map[string]any) string {
	if op == nil {
		return ""
	}
	rb, _ := op["requestBody"].(map[string]any)
	content, _ := rb["content"].(map[string]any)
	for _, v := range content {
		media, _ := v.(map[string]any)
		s, _ := media["schema"].(map[string]any)
		ref, _ := s["$ref"].(string)
		if ref != "" {
			return ref[strings.LastIndex(ref, "/")+1:]
		}
	}
	return ""
}
