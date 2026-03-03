package adminapi

import (
	"encoding/base64"
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

func TestParseBasicAuth(t *testing.T) {
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
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", nil)
		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", fiber.StatusUnauthorized, resp.StatusCode)
		}
	})

	t.Run("HeaderWithoutBasicPrefix", func(t *testing.T) {
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer token")
		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", fiber.StatusUnauthorized, resp.StatusCode)
		}
	})

	t.Run("InvalidBase64Encoding", func(t *testing.T) {
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Basic !@#invalidbase64")
		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", fiber.StatusUnauthorized, resp.StatusCode)
		}
	})

	t.Run("MissingColonInDecodedCredentials", func(t *testing.T) {
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", nil)
		encoded := base64.StdEncoding.EncodeToString([]byte("usernamepassword"))
		req.Header.Set("Authorization", "Basic "+encoded)
		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", fiber.StatusUnauthorized, resp.StatusCode)
		}
	})

	t.Run("ValidCredentials", func(t *testing.T) {
		app := setupApp()
		req := httptest.NewRequest("GET", "/test", nil)
		encoded := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
		req.Header.Set("Authorization", "Basic "+encoded)
		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Expected status %d, got %d", fiber.StatusOK, resp.StatusCode)
		}
	})
}

func TestAuthMiddleware(t *testing.T) {
	t.Run("CountError", func(t *testing.T) {
		mockStore := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 0, fiber.ErrInternalServerError
			},
		}
		app := fiber.New()
		app.Use(authMiddleware(mockStore))
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusInternalServerError {
			t.Errorf("Expected status %d, got %d", fiber.StatusInternalServerError, resp.StatusCode)
		}
	})

	t.Run("CountZero", func(t *testing.T) {
		mockStore := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 0, nil
			},
		}
		app := fiber.New()
		app.Use(authMiddleware(mockStore))
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Expected status %d, got %d", fiber.StatusOK, resp.StatusCode)
		}
	})

	t.Run("CountGreaterThanZero_MissingAuth", func(t *testing.T) {
		mockStore := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
		}
		app := fiber.New()
		app.Use(authMiddleware(mockStore))
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", fiber.StatusUnauthorized, resp.StatusCode)
		}

		wwwAuth := resp.Header.Get("WWW-Authenticate")
		if wwwAuth != "Basic realm=admin" {
			t.Errorf("Expected WWW-Authenticate header 'Basic realm=admin', got '%s'", wwwAuth)
		}
	})

	t.Run("CountGreaterThanZero_InvalidCredentials", func(t *testing.T) {
		mockStore := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
			AuthenticateFunc: func(username, password string) (*model.User, error) {
				return nil, fiber.ErrUnauthorized
			},
		}
		app := fiber.New()
		app.Use(authMiddleware(mockStore))
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		encoded := base64.StdEncoding.EncodeToString([]byte("admin:wrongpassword"))
		req.Header.Set("Authorization", "Basic "+encoded)
		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", fiber.StatusUnauthorized, resp.StatusCode)
		}

		wwwAuth := resp.Header.Get("WWW-Authenticate")
		if wwwAuth != "Basic realm=admin" {
			t.Errorf("Expected WWW-Authenticate header 'Basic realm=admin', got '%s'", wwwAuth)
		}
	})

	t.Run("CountGreaterThanZero_ValidCredentials", func(t *testing.T) {
		mockStore := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
			AuthenticateFunc: func(username, password string) (*model.User, error) {
				return &model.User{Username: username}, nil
			},
		}
		app := fiber.New()
		app.Use(authMiddleware(mockStore))
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		encoded := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
		req.Header.Set("Authorization", "Basic "+encoded)
		resp, _ := app.Test(req)

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Expected status %d, got %d", fiber.StatusOK, resp.StatusCode)
		}
	})
}
