package adminapi

import (
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// authMiddleware enforces optional authentication for admin API routes.
// If there are no users in storage, all requests are allowed.
// If there is at least one user, it requires HTTP Basic authentication
// and validates credentials using UsersStore.
func authMiddleware(users model.UsersStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		// If no users are configured, allow access
		count, err := users.Count()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(
				fiber.Map{
					"error":             "server_error",
					"error_description": err.Error(),
				},
			)
		}
		if count == 0 {
			return c.Next()
		}

		// Require Basic auth
		username, password, ok := parseBasicAuth(c)
		if !ok {
			c.Set("WWW-Authenticate", "Basic realm=admin")
			return c.Status(fiber.StatusUnauthorized).JSON(
				fiber.Map{
					"error":             "invalid_client",
					"error_description": "missing credentials",
				},
			)
		}
		// Validate credentials
		if _, err := users.Authenticate(username, password); err != nil {
			c.Set("WWW-Authenticate", "Basic realm=admin")
			return c.Status(fiber.StatusUnauthorized).JSON(
				fiber.Map{
					"error":             "invalid_client",
					"error_description": "invalid credentials",
				},
			)
		}
		// Store authenticated username for actor extraction
		SetAuthUsername(c, username)
		// All good
		return c.Next()
	}
}

// parseBasicAuth extracts Basic auth credentials from request headers
func parseBasicAuth(c fiber.Ctx) (username, password string, ok bool) {
	auth := string(c.Request().Header.Peek("Authorization"))
	if auth == "" {
		return "", "", false
	}
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return "", "", false
	}
	b, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return "", "", false
	}
	creds := string(b)
	before, after, ok0 := strings.Cut(creds, ":")
	if !ok0 {
		return "", "", false
	}
	return before, after, true
}

// localsKeyServiceToken is the key used to store, in Fiber's Locals, the
// service token a request on the service listener was authenticated with.
const localsKeyServiceToken = "service_token"

// serviceTokenAuthMiddleware enforces service token authentication on the
// service listener. Only an "Authorization: Bearer <token>" header matching
// one of tokens is accepted, also while there are no users; HTTP Basic and
// requests without credentials are refused with 401 invalid_client. The
// token's name becomes the authenticated username.
func serviceTokenAuthMiddleware(tokens ServiceTokens) fiber.Handler {
	return func(c fiber.Ctx) error {
		presented, ok := parseBearerToken(c)
		if !ok {
			c.Set("WWW-Authenticate", "Bearer realm=admin")
			return c.Status(fiber.StatusUnauthorized).JSON(
				fiber.Map{
					"error":             "invalid_client",
					"error_description": "missing credentials",
				},
			)
		}
		token, ok := tokens.find(presented)
		if !ok {
			c.Set("WWW-Authenticate", "Bearer realm=admin")
			return c.Status(fiber.StatusUnauthorized).JSON(
				fiber.Map{
					"error":             "invalid_client",
					"error_description": "invalid credentials",
				},
			)
		}
		SetAuthUsername(c, token.Name)
		c.Locals(localsKeyServiceToken, token)
		return c.Next()
	}
}

// parseBearerToken extracts a bearer token from the Authorization header. The
// scheme name is matched case-insensitively.
func parseBearerToken(c fiber.Ctx) ([]byte, bool) {
	auth := c.Request().Header.Peek("Authorization")
	const prefix = "bearer "
	if len(auth) <= len(prefix) || !strings.EqualFold(string(auth[:len(prefix)]), prefix) {
		return nil, false
	}
	return auth[len(prefix):], true
}
