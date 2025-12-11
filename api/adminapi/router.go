package adminapi

import (
	"embed"
	"net"
	neturl "net/url"
	"strconv"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse/storage/model"
)

//go:embed swagger.html swagger-users.html openapi.yaml openapi-users.yaml
var assets embed.FS

// Options controls optional features of the admin API registration.
type Options struct {
	// UsersEnabled controls whether the user management API is mounted.
	// Default behavior: enabled when left at zero value via a nil *Options in Register.
	UsersEnabled bool
	// Port, when > 0, is used to adapt the serverURL to the admin API port for docs.
	Port int
}

// Register mounts all admin API routes under the provided group.
func Register(
	r fiber.Router, serverURL string, storages model.Backends, fedEntity oidfed.FederationEntity,
	keyManagement KeyManagement, opts *Options,
) error {
	// If an admin port is provided in options, adapt the serverURL to include/override the port
	if opts != nil && opts.Port > 0 {
		serverURL = adaptServerURLPort(serverURL, opts.Port)
	}

	openapiRaw, err := assets.ReadFile("openapi.yaml")
	if err != nil {
		return errors.Wrap(err, "adminapi: failed to read openapi.yaml")
	}
	// Update servers section to point to this instance
	openapiData := updateOpenAPIServers(openapiRaw, serverURL)
	openapiData = ensureBasicAuthSecurity(openapiData)
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

	// Serve users OpenAPI under a dedicated path
	r.Get(
		"/openapi-users.yaml", func(c *fiber.Ctx) error {
			data, err := assets.ReadFile("openapi-users.yaml")
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
			}
			c.Set(fiber.HeaderContentType, "application/yaml")
			return c.Send(data)
		},
	)

	// Users docs page
	r.Get(
		"/docs/users", func(c *fiber.Ctx) error {
			html, err := assets.ReadFile("swagger-users.html")
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
			}
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTML)
			return c.Send(html)
		},
	)
	// Optional authentication middleware for all admin routes
	r.Use(authMiddleware(storages.Users))

	// Entity Configuration
	registerEntityConfiguration(r, storages.AdditionalClaims, storages.KV, fedEntity)
	// Authority Hints
	registerAuthorityHints(r, storages.AuthorityHints)
	// Keys
	registerKeys(r, keyManagement, storages.KV)
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
	registerTrustMarkTypes(r, storages.TrustMarkTypes)
	// Global Owners and Issuers
	registerTrustMarkOwners(r, storages.TrustMarkOwners, storages.TrustMarkTypes)
	registerTrustMarkIssuers(r, storages.TrustMarkIssuers, storages.TrustMarkTypes)
	registerTrustMarkIssuance(r)
	// Users management
	if opts == nil || opts.UsersEnabled {
		registerUsers(r, storages.Users)
	}
	return nil
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

// adaptServerURLPort updates or adds the port to the provided serverURL.
// If the input is invalid, it returns the original serverURL.
func adaptServerURLPort(serverURL string, port int) string {
	if len(serverURL) == 0 || port <= 0 {
		return serverURL
	}
	u, err := neturl.Parse(serverURL)
	if err != nil {
		return serverURL
	}
	host := u.Host
	if host == "" {
		return serverURL
	}
	name, _, err := net.SplitHostPort(host)
	if err != nil {
		// no port present, just append
		u.Host = net.JoinHostPort(host, strconv.Itoa(port))
		return u.String()
	}
	// replace existing port
	u.Host = net.JoinHostPort(name, strconv.Itoa(port))
	return u.String()
}

// ensureBasicAuthSecurity injects a HTTP Basic security scheme and a global security requirement
// into the OpenAPI document, if not already present.
func ensureBasicAuthSecurity(doc []byte) []byte {
	var full map[string]any
	if err := yaml.Unmarshal(doc, &full); err != nil {
		return doc
	}
	// Ensure components.securitySchemes.basicAuth
	components, _ := full["components"].(map[string]any)
	if components == nil {
		components = map[string]any{}
		full["components"] = components
	}
	securitySchemes, _ := components["securitySchemes"].(map[string]any)
	if securitySchemes == nil {
		securitySchemes = map[string]any{}
		components["securitySchemes"] = securitySchemes
	}
	if _, exists := securitySchemes["basicAuth"]; !exists {
		securitySchemes["basicAuth"] = map[string]any{
			"type":   "http",
			"scheme": "basic",
		}
	}
	// Set global security if absent
	if _, exists := full["security"]; !exists {
		full["security"] = []map[string]any{{"basicAuth": []any{}}}
	}
	res, err := yaml.Marshal(full)
	if err != nil {
		return doc
	}
	return res
}
