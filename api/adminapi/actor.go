package adminapi

import (
	"github.com/gofiber/fiber/v2"
)

// ActorSource defines the preferred source for actor extraction.
type ActorSource string

const (
	// ActorSourceBasicAuth prefers the basic auth username, falling back to header.
	ActorSourceBasicAuth ActorSource = "basic_auth"
	// ActorSourceHeader prefers the configured header, falling back to basic auth username.
	ActorSourceHeader ActorSource = "header"
)

// ActorConfig holds configuration for actor extraction.
type ActorConfig struct {
	// Header is the HTTP header name to extract the actor from.
	// Default: "X-Actor"
	Header string
	// Source is the preferred source for actor extraction.
	// Default: ActorSourceBasicAuth
	Source ActorSource
}

// localsKeyActor is the key used to store the actor in Fiber's Locals.
const localsKeyActor = "actor"

// localsKeyAuthUsername is the key used to store the authenticated username in Fiber's Locals.
const localsKeyAuthUsername = "auth_username"

// actorMiddleware creates a middleware that extracts the actor from the request
// based on the provided configuration and stores it in Fiber's Locals.
func actorMiddleware(cfg ActorConfig) fiber.Handler {
	// Set defaults if not configured
	if cfg.Header == "" {
		cfg.Header = "X-Actor"
	}
	if cfg.Source == "" {
		cfg.Source = ActorSourceBasicAuth
	}

	return func(c *fiber.Ctx) error {
		actor := extractActor(c, cfg)
		if actor != "" {
			c.Locals(localsKeyActor, actor)
		}
		return c.Next()
	}
}

// extractActor extracts the actor from the request based on configuration.
// It tries the preferred source first, then falls back to the other source.
func extractActor(c *fiber.Ctx, cfg ActorConfig) string {
	var primary, fallback string

	// Get values from both sources
	authUsername := getAuthUsername(c)
	headerValue := getHeaderValue(c, cfg.Header)

	// Determine order based on preferred source
	if cfg.Source == ActorSourceHeader {
		primary = headerValue
		fallback = authUsername
	} else {
		// Default: basic_auth first
		primary = authUsername
		fallback = headerValue
	}

	// Return primary if set, otherwise fallback
	if primary != "" {
		return primary
	}
	return fallback
}

// getAuthUsername retrieves the authenticated username from Fiber's Locals.
// This is set by the auth middleware when basic auth is used.
func getAuthUsername(c *fiber.Ctx) string {
	if username, ok := c.Locals(localsKeyAuthUsername).(string); ok {
		return username
	}
	return ""
}

// getHeaderValue retrieves the value of the specified header from the request.
func getHeaderValue(c *fiber.Ctx, header string) string {
	if header == "" {
		return ""
	}
	return string(c.Request().Header.Peek(header))
}

// GetActor retrieves the actor from Fiber's Locals.
// Returns an empty string if no actor is set.
func GetActor(c *fiber.Ctx) string {
	if actor, ok := c.Locals(localsKeyActor).(string); ok {
		return actor
	}
	return ""
}

// SetAuthUsername stores the authenticated username in Fiber's Locals.
// This should be called by the auth middleware after successful authentication.
func SetAuthUsername(c *fiber.Ctx, username string) {
	c.Locals(localsKeyAuthUsername, username)
}
