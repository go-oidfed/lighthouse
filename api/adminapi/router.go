package adminapi

import (
	"embed"

	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse/storage/model"
)

//go:embed swagger.html openapi.yaml
var assets embed.FS

// Register mounts all admin API routes under the provided group.
func Register(r fiber.Router, serverURL string, storages model.Backends) error {
	openapiRaw, err := assets.ReadFile("openapi.yaml")
	if err != nil {
		return errors.Wrap(err, "adminapi: failed to read openapi.yaml")
	}
	// Update servers section to point to this instance
	openapiData := updateOpenAPIServers(openapiRaw, serverURL)
	swaggerHTML, err := assets.ReadFile("swagger.html")
	if err != nil {
		return errors.Wrap(err, "adminapi: failed to read swagger.html")
	}

	r.Get(
		"/openapi.yaml", func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderContentType, "application/yaml")
			return c.Send(openapiData)
		},
	)

	r.Get(
		"/docs", func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTML)
			return c.Send(swaggerHTML)
		},
	)
	// Entity Configuration
	registerEntityConfiguration(r, storages.AdditionalClaims, storages.KV)
	// Authority Hints
	registerAuthorityHints(r, storages.AuthorityHints)
	// Keys
	registerKeys(r)
	// Entity Configuration Trust Marks
	registerEntityTrustMarks(r)
	// Entity Configuration Metadata and Policies
	registerEntityMetadata(r)
	registerEntityMetadataPolicies(r)
	// Subordinates
	registerSubordinates(r)
	// Subordinate metadata, policies, constraints, keys
	registerSubordinateMetadata(r)
	registerSubordinateMetadataPolicies(r)
	registerSubordinateConstraints(r)
	registerSubordinateKeys(r)
	registerSubordinateMetadataPolicyCrit(r)
	// Trust Mark Types and Issuance
	registerTrustMarkTypes(r)
	registerTrustMarkIssuance(r)
	return nil
}

type openAPIServers struct {
	Servers []struct {
		URL         string `yaml:"url"`
		Description string `yaml:"description,omitempty"`
	} `yaml:"servers"`
}

func updateOpenAPIServers(doc []byte, serverURL string) []byte {
	if len(serverURL) == 0 {
		return doc
	}
	// Unmarshal full doc
	var full map[string]any
	if err := yaml.Unmarshal(doc, &full); err != nil {
		return doc
	}
	full["servers"] = []map[string]any{
		{
			"url":         serverURL,
			"description": "This instance",
		},
	}
	res, err := yaml.Marshal(full)
	if err != nil {
		return doc
	}
	return res
}
