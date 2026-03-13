package adminapi

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-oidfed/lighthouse/storage/model"
	"github.com/gofiber/fiber/v2"
)

// mockUsersStore is a custom mock for model.UsersStore
type mockUsersStore struct {
	CountFunc        func() (int64, error)
	ListFunc         func() ([]model.User, error)
	GetFunc          func(username string) (*model.User, error)
	CreateFunc       func(username, password, displayName string) (*model.User, error)
	UpdateFunc       func(username string, displayName *string, newPassword *string, disabled *bool) (*model.User, error)
	DeleteFunc       func(username string) error
	AuthenticateFunc func(username, password string) (*model.User, error)
}

func (m *mockUsersStore) Count() (int64, error) {
	if m.CountFunc != nil {
		return m.CountFunc()
	}
	return 0, nil
}

func (m *mockUsersStore) List() ([]model.User, error) {
	if m.ListFunc != nil {
		return m.ListFunc()
	}
	return nil, nil
}

func (m *mockUsersStore) Get(username string) (*model.User, error) {
	if m.GetFunc != nil {
		return m.GetFunc(username)
	}
	return nil, nil
}

func (m *mockUsersStore) Create(username, password, displayName string) (*model.User, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(username, password, displayName)
	}
	return nil, nil
}

func (m *mockUsersStore) Update(username string, displayName *string, newPassword *string, disabled *bool) (*model.User, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(username, displayName, newPassword, disabled)
	}
	return nil, nil
}

func (m *mockUsersStore) Delete(username string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(username)
	}
	return nil
}

func (m *mockUsersStore) Authenticate(username, password string) (*model.User, error) {
	if m.AuthenticateFunc != nil {
		return m.AuthenticateFunc(username, password)
	}
	return nil, nil
}

// readJSON is a test helper that reads and unmarshals a JSON response body.
func readJSON(t *testing.T, resp *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.NewDecoder(resp.Result().Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	return result
}

// readJSONFromHTTP is a helper that reads JSON from a standard *http.Response (fiber.App.Test returns this).
func readJSONFromHTTP(t *testing.T, body io.ReadCloser) map[string]any {
	t.Helper()
	defer body.Close()
	var result map[string]any
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	return result
}

// basicAuthHeader returns a properly encoded Basic auth header value.
func basicAuthHeader(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

func TestParseBasicAuth(t *testing.T) {
	t.Parallel()
	setupApp := func() *fiber.App {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			username, password, ok := parseBasicAuth(c)
			if !ok {
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			return c.JSON(fiber.Map{"username": username, "password": password})
		})
		return app
	}

	t.Run("MissingAuthorizationHeader", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusUnauthorized)
	})

	t.Run("HeaderWithoutBasicPrefix", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", "Bearer token")
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusUnauthorized)
	})

	t.Run("InvalidBase64Encoding", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", "Basic !@#invalidbase64")
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusUnauthorized)
	})

	t.Run("MissingColonInDecodedCredentials", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		encoded := base64.StdEncoding.EncodeToString([]byte("usernamepassword"))
		req.Header.Set("Authorization", "Basic "+encoded)
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusUnauthorized)
	})

	t.Run("ValidCredentials", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", basicAuthHeader("admin", "secret123"))
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusOK)
		body := readJSONFromHTTP(t, resp.Body)
		if body["username"] != "admin" {
			t.Errorf("Expected username 'admin', got '%s'", body["username"])
		}
		if body["password"] != "secret123" {
			t.Errorf("Expected password 'secret123', got '%s'", body["password"])
		}
	})

	t.Run("EmptyPassword", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", basicAuthHeader("admin", ""))
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusOK)
		body := readJSONFromHTTP(t, resp.Body)
		if body["username"] != "admin" {
			t.Errorf("Expected username 'admin', got '%s'", body["username"])
		}
		if body["password"] != "" {
			t.Errorf("Expected empty password, got '%s'", body["password"])
		}
	})

	t.Run("EmptyUsername", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", basicAuthHeader("", "password"))
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusOK)
		body := readJSONFromHTTP(t, resp.Body)
		if body["username"] != "" {
			t.Errorf("Expected empty username, got '%s'", body["username"])
		}
	})

	t.Run("PasswordWithColons", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", basicAuthHeader("admin", "pass:word:123"))
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusOK)
		body := readJSONFromHTTP(t, resp.Body)
		if body["username"] != "admin" {
			t.Errorf("Expected username 'admin', got '%s'", body["username"])
		}
		if body["password"] != "pass:word:123" {
			t.Errorf("Expected password 'pass:word:123', got '%s'", body["password"])
		}
	})

	t.Run("CaseSensitivePrefix", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		encoded := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
		req.Header.Set("Authorization", "basic "+encoded) // lowercase "basic"
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusUnauthorized)
	})

	t.Run("EmptyAuthorizationHeader", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", "")
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusUnauthorized)
	})

	t.Run("BasicPrefixOnly", func(t *testing.T) {
			t.Parallel()
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", "Basic ")
		resp, _ := app.Test(req)

		// "Basic " with empty payload should fail base64 decode or missing colon
		assertStatus(t, resp, fiber.StatusUnauthorized)
	})
}

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()
	// setupAuthApp creates a Fiber app with the authMiddleware and a test endpoint.
	setupAuthApp := func(store model.UsersStore) *fiber.App {
		app := fiber.New()
		app.Use(authMiddleware(store))
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})
		return app
	}

	t.Run("CountError", func(t *testing.T) {
			t.Parallel()
		app := setupAuthApp(&mockUsersStore{
			CountFunc: func() (int64, error) {
				return 0, fiber.ErrInternalServerError
			},
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusInternalServerError)
		body := readJSONFromHTTP(t, resp.Body)
		if body["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", body["error"])
		}
	})

	t.Run("CountZero_NoAuthRequired", func(t *testing.T) {
			t.Parallel()
		app := setupAuthApp(&mockUsersStore{
			CountFunc: func() (int64, error) {
				return 0, nil
			},
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusOK)
	})

	t.Run("CountGreaterThanZero_MissingAuth", func(t *testing.T) {
			t.Parallel()
		app := setupAuthApp(&mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusUnauthorized)

		wwwAuth := resp.Header.Get("WWW-Authenticate")
		if wwwAuth != "Basic realm=admin" {
			t.Errorf("Expected WWW-Authenticate header 'Basic realm=admin', got '%s'", wwwAuth)
		}

		body := readJSONFromHTTP(t, resp.Body)
		if body["error"] != "invalid_client" {
			t.Errorf("Expected error 'invalid_client', got '%s'", body["error"])
		}
		if body["error_description"] != "missing credentials" {
			t.Errorf("Expected error_description 'missing credentials', got '%s'", body["error_description"])
		}
	})

	t.Run("CountGreaterThanZero_InvalidCredentials", func(t *testing.T) {
			t.Parallel()
		app := setupAuthApp(&mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
			AuthenticateFunc: func(username, password string) (*model.User, error) {
				return nil, fiber.ErrUnauthorized
			},
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", basicAuthHeader("admin", "wrongpassword"))
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusUnauthorized)

		wwwAuth := resp.Header.Get("WWW-Authenticate")
		if wwwAuth != "Basic realm=admin" {
			t.Errorf("Expected WWW-Authenticate header 'Basic realm=admin', got '%s'", wwwAuth)
		}

		body := readJSONFromHTTP(t, resp.Body)
		if body["error"] != "invalid_client" {
			t.Errorf("Expected error 'invalid_client', got '%s'", body["error"])
		}
		if body["error_description"] != "invalid credentials" {
			t.Errorf("Expected error_description 'invalid credentials', got '%s'", body["error_description"])
		}
	})

	t.Run("CountGreaterThanZero_ValidCredentials", func(t *testing.T) {
			t.Parallel()
		app := setupAuthApp(&mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
			AuthenticateFunc: func(username, password string) (*model.User, error) {
				return &model.User{Username: username}, nil
			},
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", basicAuthHeader("admin", "secret123"))
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusOK)
	})

	t.Run("CountGreaterThanZero_DisabledUser", func(t *testing.T) {
			t.Parallel()
		// Even though the user is disabled, authMiddleware does not check the disabled field.
		// This test documents that behavior. If disabled-user checking is added later, update here.
		app := setupAuthApp(&mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
			AuthenticateFunc: func(username, password string) (*model.User, error) {
				return &model.User{Username: username, Disabled: true}, nil
			},
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", basicAuthHeader("admin", "secret123"))
		resp, _ := app.Test(req)

		// Currently allowed because authMiddleware only checks Authenticate error
		assertStatus(t, resp, fiber.StatusOK)
	})

	t.Run("MultipleUsersInStore", func(t *testing.T) {
			t.Parallel()
		app := setupAuthApp(&mockUsersStore{
			CountFunc: func() (int64, error) {
				return 5, nil
			},
			AuthenticateFunc: func(username, password string) (*model.User, error) {
				if username == "user2" && password == "pass2" {
					return &model.User{Username: username}, nil
				}
				return nil, fiber.ErrUnauthorized
			},
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", basicAuthHeader("user2", "pass2"))
		resp, _ := app.Test(req)

		assertStatus(t, resp, fiber.StatusOK)
	})
}
