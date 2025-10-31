package adminapi

import (
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// authMiddleware enforces optional authentication for admin API routes.
// If there are no users in storage, all requests are allowed.
// If there is at least one user, it requires HTTP Basic authentication
// and validates credentials using UsersStore.
func authMiddleware(users model.UsersStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// If no users are configured, allow access
		count, err := users.Count()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server_error", "error_description": err.Error()})
		}
		if count == 0 {
			return c.Next()
		}

		// Require Basic auth
		username, password, ok := parseBasicAuth(c)
		if !ok {
			c.Set("WWW-Authenticate", "Basic realm=admin")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_client", "error_description": "missing credentials"})
		}
		// Validate credentials
		if _, err := users.Authenticate(username, password); err != nil {
			c.Set("WWW-Authenticate", "Basic realm=admin")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_client", "error_description": "invalid credentials"})
		}
		// All good
		return c.Next()
	}
}

// parseBasicAuth extracts Basic auth credentials from request headers
func parseBasicAuth(c *fiber.Ctx) (username, password string, ok bool) {
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
	i := strings.IndexByte(creds, ':')
	if i < 0 {
		return "", "", false
	}
	return creds[:i], creds[i+1:], true
}
