package adminapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// setupActorTestApp creates a Fiber app with the actor middleware and a test endpoint
// that returns the extracted actor value.
func setupActorTestApp(cfg ActorConfig, preMiddleware ...fiber.Handler) *fiber.App {
	app := fiber.New()
	for _, mw := range preMiddleware {
		app.Use(mw)
	}
	app.Use(actorMiddleware(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		actor := GetActor(c)
		return c.JSON(fiber.Map{"actor": actor})
	})
	return app
}

// simulateAuthMiddleware returns a middleware that sets the auth username in Locals,
// simulating what the real authMiddleware does after successful authentication.
func simulateAuthMiddleware(username string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if username != "" {
			SetAuthUsername(c, username)
		}
		return c.Next()
	}
}

func TestActorMiddleware_Defaults(t *testing.T) {
	t.Parallel()

	t.Run("EmptyConfig/DefaultsToBasicAuthAndXActor", func(t *testing.T) {
		t.Parallel()
		app := setupActorTestApp(ActorConfig{}, simulateAuthMiddleware("admin"))

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, http.StatusOK)

		// With basic_auth as default source and auth username set, actor should be "admin"
		if string(body) != `{"actor":"admin"}` {
			t.Errorf("Expected actor 'admin', got: %s", body)
		}
	})

	t.Run("EmptyConfig/NoAuthNoHeader/EmptyActor", func(t *testing.T) {
		t.Parallel()
		app := setupActorTestApp(ActorConfig{})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, http.StatusOK)

		if string(body) != `{"actor":""}` {
			t.Errorf("Expected empty actor, got: %s", body)
		}
	})
}

func TestActorMiddleware_BasicAuthSource(t *testing.T) {
	t.Parallel()

	t.Run("AuthOnly/ReturnsUsername", func(t *testing.T) {
		t.Parallel()
		app := setupActorTestApp(
			ActorConfig{Source: ActorSourceBasicAuth},
			simulateAuthMiddleware("alice"),
		)

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, http.StatusOK)

		if string(body) != `{"actor":"alice"}` {
			t.Errorf("Expected actor 'alice', got: %s", body)
		}
	})

	t.Run("HeaderOnly/FallsBackToHeader", func(t *testing.T) {
		t.Parallel()
		app := setupActorTestApp(ActorConfig{Source: ActorSourceBasicAuth})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("X-Actor", "proxy-user")
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, http.StatusOK)

		if string(body) != `{"actor":"proxy-user"}` {
			t.Errorf("Expected actor 'proxy-user', got: %s", body)
		}
	})

	t.Run("BothSet/PrefersBasicAuth", func(t *testing.T) {
		t.Parallel()
		app := setupActorTestApp(
			ActorConfig{Source: ActorSourceBasicAuth},
			simulateAuthMiddleware("alice"),
		)

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("X-Actor", "proxy-user")
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, http.StatusOK)

		if string(body) != `{"actor":"alice"}` {
			t.Errorf("Expected actor 'alice' (basic auth preferred), got: %s", body)
		}
	})
}

func TestActorMiddleware_HeaderSource(t *testing.T) {
	t.Parallel()

	t.Run("HeaderOnly/ReturnsHeader", func(t *testing.T) {
		t.Parallel()
		app := setupActorTestApp(ActorConfig{Source: ActorSourceHeader})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("X-Actor", "proxy-user")
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, http.StatusOK)

		if string(body) != `{"actor":"proxy-user"}` {
			t.Errorf("Expected actor 'proxy-user', got: %s", body)
		}
	})

	t.Run("AuthOnly/FallsBackToAuth", func(t *testing.T) {
		t.Parallel()
		app := setupActorTestApp(
			ActorConfig{Source: ActorSourceHeader},
			simulateAuthMiddleware("alice"),
		)

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, http.StatusOK)

		if string(body) != `{"actor":"alice"}` {
			t.Errorf("Expected actor 'alice' (fallback to auth), got: %s", body)
		}
	})

	t.Run("BothSet/PrefersHeader", func(t *testing.T) {
		t.Parallel()
		app := setupActorTestApp(
			ActorConfig{Source: ActorSourceHeader},
			simulateAuthMiddleware("alice"),
		)

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("X-Actor", "proxy-user")
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, http.StatusOK)

		if string(body) != `{"actor":"proxy-user"}` {
			t.Errorf("Expected actor 'proxy-user' (header preferred), got: %s", body)
		}
	})
}

func TestActorMiddleware_CustomHeader(t *testing.T) {
	t.Parallel()

	t.Run("CustomHeader/Extracted", func(t *testing.T) {
		t.Parallel()
		app := setupActorTestApp(ActorConfig{Header: "X-Remote-User"})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("X-Remote-User", "custom-actor")
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, http.StatusOK)

		if string(body) != `{"actor":"custom-actor"}` {
			t.Errorf("Expected actor 'custom-actor', got: %s", body)
		}
	})

	t.Run("CustomHeader/DefaultHeaderIgnored", func(t *testing.T) {
		t.Parallel()
		app := setupActorTestApp(ActorConfig{Header: "X-Remote-User"})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("X-Actor", "wrong-actor")
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, http.StatusOK)

		// X-Actor should be ignored since custom header is configured
		if string(body) != `{"actor":""}` {
			t.Errorf("Expected empty actor (default header ignored), got: %s", body)
		}
	})
}

func TestGetActor_NoMiddleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		actor := GetActor(c)
		return c.JSON(fiber.Map{"actor": actor})
	})

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	resp, body := doRequest(t, app, req)
	requireStatus(t, resp, http.StatusOK)

	if string(body) != `{"actor":""}` {
		t.Errorf("Expected empty actor without middleware, got: %s", body)
	}
}

func TestSetAuthUsername_And_GetAuthUsername(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		SetAuthUsername(c, "testuser")
		username := getAuthUsername(c)
		return c.JSON(fiber.Map{"username": username})
	})

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	resp, body := doRequest(t, app, req)
	requireStatus(t, resp, http.StatusOK)

	if string(body) != `{"username":"testuser"}` {
		t.Errorf("Expected username 'testuser', got: %s", body)
	}
}
