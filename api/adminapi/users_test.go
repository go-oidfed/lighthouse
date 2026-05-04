package adminapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-oidfed/lighthouse/storage/model"
	"github.com/gofiber/fiber/v2"
)

// setupUsersApp creates a Fiber app with users routes registered and no auth middleware.
func setupUsersApp(t *testing.T, store model.UsersStore) *fiber.App {
	t.Helper()
	app := fiber.New()
	registerUsers(app.Group("/api/v1/admin"), store)
	return app
}

// doJSONRequest is a helper to execute a request against the test app and return the response.
func doJSONRequest(t *testing.T, app *fiber.App, method, path string, body any) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	return resp
}

// decodeJSON reads and decodes a JSON response body.
func decodeJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	return result
}

// decodeJSONArray reads and decodes a JSON array response body.
func decodeJSONArray(t *testing.T, resp *http.Response) []any {
	t.Helper()
	defer resp.Body.Close()
	var result []any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON array response: %v", err)
	}
	return result
}

// --- Test: GET /api/v1/admin/users/ ---

func TestListUsers(t *testing.T) {
	t.Parallel()
	t.Run("Success_Empty", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			ListFunc: func() ([]model.User, error) {
				return []model.User{}, nil
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "GET", "/api/v1/admin/users/", nil)
		assertStatus(t, resp, fiber.StatusOK)

		list := decodeJSONArray(t, resp)
		if len(list) != 0 {
			t.Errorf("Expected empty list, got %d items", len(list))
		}
	})

	t.Run("Success_MultipleUsers", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			ListFunc: func() ([]model.User, error) {
				return []model.User{
					{ID: 1, Username: "alice", DisplayName: "Alice", CreatedAt: time.Now(), UpdatedAt: time.Now()},
					{ID: 2, Username: "bob", DisplayName: "Bob", CreatedAt: time.Now(), UpdatedAt: time.Now()},
				}, nil
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "GET", "/api/v1/admin/users/", http.NoBody)
		assertStatus(t, resp, fiber.StatusOK)

		list := decodeJSONArray(t, resp)
		if len(list) != 2 {
			t.Errorf("Expected 2 users, got %d", len(list))
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			ListFunc: func() ([]model.User, error) {
				return nil, fiber.ErrInternalServerError
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "GET", "/api/v1/admin/users/", nil)
		assertStatus(t, resp, fiber.StatusInternalServerError)

		body := decodeJSON(t, resp)
		if body["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", body["error"])
		}
	})
}

// --- Test: POST /api/v1/admin/users/ ---

func TestCreateUser(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			CreateFunc: func(username, _, displayName string) (*model.User, error) {
				return &model.User{
					ID:          1,
					Username:    username,
					DisplayName: displayName,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}, nil
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "POST", "/api/v1/admin/users/", map[string]string{
			"username":     "alice",
			"password":     "strongpass",
			"display_name": "Alice",
		})
		assertStatus(t, resp, fiber.StatusCreated)

		body := decodeJSON(t, resp)
		if body["username"] != "alice" {
			t.Errorf("Expected username 'alice', got '%s'", body["username"])
		}
		if body["display_name"] != "Alice" {
			t.Errorf("Expected display_name 'Alice', got '%s'", body["display_name"])
		}
	})

	t.Run("MissingUsername", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "POST", "/api/v1/admin/users/", map[string]string{
			"password": "strongpass",
		})
		assertStatus(t, resp, fiber.StatusBadRequest)

		body := decodeJSON(t, resp)
		if body["error"] != "invalid_request" {
			t.Errorf("Expected error 'invalid_request', got '%s'", body["error"])
		}
	})

	t.Run("MissingPassword", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "POST", "/api/v1/admin/users/", map[string]string{
			"username": "alice",
		})
		assertStatus(t, resp, fiber.StatusBadRequest)

		body := decodeJSON(t, resp)
		if body["error"] != "invalid_request" {
			t.Errorf("Expected error 'invalid_request', got '%s'", body["error"])
		}
	})

	t.Run("EmptyBody", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "POST", "/api/v1/admin/users/", map[string]string{})
		assertStatus(t, resp, fiber.StatusBadRequest)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{}
		app := setupUsersApp(t, store)

		req := httptest.NewRequest("POST", "/api/v1/admin/users/", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		assertStatus(t, resp, fiber.StatusBadRequest)
	})

	t.Run("ConflictAlreadyExists", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			CreateFunc: func(_, _, _ string) (*model.User, error) {
				return nil, model.AlreadyExistsError("user already exists")
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "POST", "/api/v1/admin/users/", map[string]string{
			"username": "alice",
			"password": "strongpass",
		})
		assertStatus(t, resp, fiber.StatusConflict)

		body := decodeJSON(t, resp)
		if body["error"] != "invalid_request" {
			t.Errorf("Expected error 'invalid_request', got '%s'", body["error"])
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			CreateFunc: func(_, _, _ string) (*model.User, error) {
				return nil, fiber.ErrInternalServerError
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "POST", "/api/v1/admin/users/", map[string]string{
			"username": "alice",
			"password": "strongpass",
		})
		assertStatus(t, resp, fiber.StatusInternalServerError)

		body := decodeJSON(t, resp)
		if body["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", body["error"])
		}
	})
}

// --- Test: GET /api/v1/admin/users/:username ---

func TestGetUser(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			GetFunc: func(username string) (*model.User, error) {
				return &model.User{
					ID:          1,
					Username:    username,
					DisplayName: "Alice",
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}, nil
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "GET", "/api/v1/admin/users/alice", nil)
		assertStatus(t, resp, fiber.StatusOK)

		body := decodeJSON(t, resp)
		if body["username"] != "alice" {
			t.Errorf("Expected username 'alice', got '%s'", body["username"])
		}
		if body["display_name"] != "Alice" {
			t.Errorf("Expected display_name 'Alice', got '%s'", body["display_name"])
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			GetFunc: func(_ string) (*model.User, error) {
				return nil, model.NotFoundError("user not found")
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "GET", "/api/v1/admin/users/unknown", nil)
		assertStatus(t, resp, fiber.StatusNotFound)

		body := decodeJSON(t, resp)
		if body["error"] != "not_found" {
			t.Errorf("Expected error 'not_found', got '%s'", body["error"])
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			GetFunc: func(_ string) (*model.User, error) {
				return nil, fiber.ErrInternalServerError
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "GET", "/api/v1/admin/users/alice", nil)
		assertStatus(t, resp, fiber.StatusInternalServerError)

		body := decodeJSON(t, resp)
		if body["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", body["error"])
		}
	})
}

// --- Test: PUT /api/v1/admin/users/:username ---

func TestUpdateUser(t *testing.T) {
	t.Parallel()
	t.Run("Success_DisplayName", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			UpdateFunc: func(username string, displayName *string, _ *string, _ *bool) (*model.User, error) {
				dn := "Alice Updated"
				if displayName != nil {
					dn = *displayName
				}
				return &model.User{
					ID:          1,
					Username:    username,
					DisplayName: dn,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}, nil
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "PUT", "/api/v1/admin/users/alice", map[string]string{
			"display_name": "Alice Updated",
		})
		assertStatus(t, resp, fiber.StatusOK)

		body := decodeJSON(t, resp)
		if body["display_name"] != "Alice Updated" {
			t.Errorf("Expected display_name 'Alice Updated', got '%s'", body["display_name"])
		}
	})

	t.Run("Success_Password", func(t *testing.T) {
		t.Parallel()
		updateCalled := false
		store := &mockUsersStore{
			UpdateFunc: func(username string, _ *string, newPassword *string, _ *bool) (*model.User, error) {
				updateCalled = true
				if newPassword == nil {
					t.Error("Expected newPassword to be non-nil")
				}
				return &model.User{
					ID:       1,
					Username: username,
				}, nil
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "PUT", "/api/v1/admin/users/alice", map[string]string{
			"password": "newpass123",
		})
		assertStatus(t, resp, fiber.StatusOK)
		if !updateCalled {
			t.Error("Expected Update to be called")
		}
	})

	t.Run("Success_Disabled", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			UpdateFunc: func(username string, _ *string, _ *string, disabled *bool) (*model.User, error) {
				if disabled == nil || !*disabled {
					t.Error("Expected disabled to be true")
				}
				return &model.User{
					ID:       1,
					Username: username,
					Disabled: true,
				}, nil
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "PUT", "/api/v1/admin/users/alice", map[string]any{
			"disabled": true,
		})
		assertStatus(t, resp, fiber.StatusOK)

		body := decodeJSON(t, resp)
		if body["disabled"] != true {
			t.Errorf("Expected disabled true, got %v", body["disabled"])
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			UpdateFunc: func(_ string, _ *string, _ *string, _ *bool) (*model.User, error) {
				return nil, model.NotFoundError("user not found")
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "PUT", "/api/v1/admin/users/unknown", map[string]string{
			"display_name": "Test",
		})
		assertStatus(t, resp, fiber.StatusNotFound)

		body := decodeJSON(t, resp)
		if body["error"] != "not_found" {
			t.Errorf("Expected error 'not_found', got '%s'", body["error"])
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{}
		app := setupUsersApp(t, store)

		req := httptest.NewRequest("PUT", "/api/v1/admin/users/alice", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		assertStatus(t, resp, fiber.StatusBadRequest)
	})

	t.Run("InternalError", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			UpdateFunc: func(_ string, _ *string, _ *string, _ *bool) (*model.User, error) {
				return nil, fiber.ErrInternalServerError
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "PUT", "/api/v1/admin/users/alice", map[string]string{
			"display_name": "Test",
		})
		assertStatus(t, resp, fiber.StatusInternalServerError)

		body := decodeJSON(t, resp)
		if body["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", body["error"])
		}
	})
}

// --- Test: DELETE /api/v1/admin/users/:username ---

func TestDeleteUser(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		deleteCalled := false
		store := &mockUsersStore{
			DeleteFunc: func(username string) error {
				deleteCalled = true
				if username != "alice" {
					t.Errorf("Expected username 'alice', got '%s'", username)
				}
				return nil
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "DELETE", "/api/v1/admin/users/alice", nil)
		assertStatus(t, resp, fiber.StatusNoContent)

		if !deleteCalled {
			t.Error("Expected Delete to be called")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			DeleteFunc: func(_ string) error {
				return model.NotFoundError("user not found")
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "DELETE", "/api/v1/admin/users/unknown", nil)
		assertStatus(t, resp, fiber.StatusNotFound)

		body := decodeJSON(t, resp)
		if body["error"] != "not_found" {
			t.Errorf("Expected error 'not_found', got '%s'", body["error"])
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			DeleteFunc: func(_ string) error {
				return fiber.ErrInternalServerError
			},
		}
		app := setupUsersApp(t, store)
		resp := doJSONRequest(t, app, "DELETE", "/api/v1/admin/users/alice", nil)
		assertStatus(t, resp, fiber.StatusInternalServerError)

		body := decodeJSON(t, resp)
		if body["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", body["error"])
		}
	})
}

// --- Test: Users endpoints with auth middleware integration ---

func TestUsersWithAuthMiddleware(t *testing.T) {
	t.Parallel()
	// This test verifies that when auth middleware is applied, users endpoints
	// correctly require auth when users exist.
	newAppWithAuth := func(store *mockUsersStore) *fiber.App {
		app := fiber.New()
		grp := app.Group("/api/v1/admin")
		grp.Use(authMiddleware(store))
		registerUsers(grp, store)
		return app
	}

	t.Run("NoUsers_AccessWithoutAuth", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 0, nil
			},
			ListFunc: func() ([]model.User, error) {
				return []model.User{}, nil
			},
		}
		app := newAppWithAuth(store)
		resp := doJSONRequest(t, app, "GET", "/api/v1/admin/users/", nil)
		assertStatus(t, resp, fiber.StatusOK)
	})

	t.Run("WithUsers_RequiresAuth", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
		}
		app := newAppWithAuth(store)
		resp := doJSONRequest(t, app, "GET", "/api/v1/admin/users/", nil)
		assertStatus(t, resp, fiber.StatusUnauthorized)
	})

	t.Run("WithUsers_AuthenticatedAccess", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
			AuthenticateFunc: func(username, password string) (*model.User, error) {
				if username == "admin" && password == "pass" {
					return &model.User{Username: username}, nil
				}
				return nil, fiber.ErrUnauthorized
			},
			ListFunc: func() ([]model.User, error) {
				return []model.User{{ID: 1, Username: "admin"}}, nil
			},
		}
		app := newAppWithAuth(store)

		req := httptest.NewRequest("GET", "/api/v1/admin/users/", http.NoBody)
		req.Header.Set("Authorization", basicAuthHeader("admin", "pass"))
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		assertStatus(t, resp, fiber.StatusOK)
	})

	t.Run("WithUsers_CreateRequiresAuth", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
		}
		app := newAppWithAuth(store)
		resp := doJSONRequest(t, app, "POST", "/api/v1/admin/users/", map[string]string{
			"username": "newuser",
			"password": "pass",
		})
		assertStatus(t, resp, fiber.StatusUnauthorized)
	})
}
