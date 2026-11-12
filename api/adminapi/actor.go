package adminapi

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage/model"
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

	return func(c fiber.Ctx) error {
		actor := extractActor(c, cfg)
		if actor != "" {
			c.Locals(localsKeyActor, actor)
		}
		return c.Next()
	}
}

// extractActor extracts the actor from the request based on configuration.
// It tries the preferred source first, then falls back to the other source.
func extractActor(c fiber.Ctx, cfg ActorConfig) string {
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
func getAuthUsername(c fiber.Ctx) string {
	if username, ok := c.Locals(localsKeyAuthUsername).(string); ok {
		return username
	}
	return ""
}

// getHeaderValue retrieves the value of the specified header from the request.
func getHeaderValue(c fiber.Ctx, header string) string {
	if header == "" {
		return ""
	}
	return string(c.Request().Header.Peek(header))
}

// GetActor retrieves the actor from Fiber's Locals.
// Returns an empty string if no actor is set.
func GetActor(c fiber.Ctx) string {
	if actor, ok := c.Locals(localsKeyActor).(string); ok {
		return actor
	}
	return ""
}

// SetAuthUsername stores the authenticated username in Fiber's Locals.
// This should be called by the auth middleware after successful authentication.
func SetAuthUsername(c fiber.Ctx, username string) {
	c.Locals(localsKeyAuthUsername, username)
}

// serviceActorHeader names the user a service token with act_for_users acts
// for on the service listener.
const serviceActorHeader = "X-Actor"

// serviceActorMiddleware sets the actor on the service listener. The actor is
// the service token's name, unless the token may act for users and the
// request names a user in X-Actor: that user must exist and be enabled, and
// their stored username is recorded; otherwise the request is refused with
// 400 invalid_actor before any handler runs. While there are no users no name
// can be honoured, so X-Actor is always refused then. Tokens without
// act_for_users ignore X-Actor. The admin port's actor configuration does not
// apply here.
func serviceActorMiddleware(users model.UsersStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		actor := getAuthUsername(c)
		if token, _ := c.Locals(localsKeyServiceToken).(ServiceToken); token.ActForUsers {
			if named := getHeaderValue(c, serviceActorHeader); named != "" {
				canonical, ok, err := delegatedActor(users, named)
				if err != nil {
					return writeServerError(c, err)
				}
				if !ok {
					return writeCodedError(
						c, fiber.StatusBadRequest, "invalid_actor",
						"X-Actor must name an existing, enabled user",
					)
				}
				actor = canonical
			}
		}
		if actor != "" {
			c.Locals(localsKeyActor, actor)
		}
		return c.Next()
	}
}

// delegatedActor reports whether a service token may act for the user name
// and returns the actor to record: the stored user's username, which the
// store may have matched case-insensitively.
func delegatedActor(users model.UsersStore, name string) (string, bool, error) {
	user, err := users.Get(name)
	if err != nil {
		if _, ok := errors.AsType[model.NotFoundError](err); ok {
			return "", false, nil
		}
		return "", false, err
	}
	if user == nil || user.Disabled || user.Username == "" {
		return "", false, nil
	}
	return user.Username, true, nil
}
