package adminapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-oidfed/lighthouse/storage/model"
	"github.com/gofiber/fiber/v3"
)

// setupUsersApp creates a Fiber app with users routes registered and no auth middleware.
func setupUsersApp(t *testing.T, store model.UsersStore) *fiber.App {
	t.Helper()
	app := fiber.New()
	registerUsers(app.Group("/api/v1/admin"), store)
	return app
}

// newJSONRequest creates an httptest.NewRequest with an optional JSON body.
// Use this to build requests for doRequest when the body needs to be marshaled from a Go value.
func newJSONRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, http.NoBody)
	}
	return req
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
		req := newJSONRequest(t, "GET", "/api/v1/admin/users/", nil)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var list []any
		if err := json.Unmarshal(body, &list); err != nil {
			t.Fatalf("failed to decode JSON array response: %v", err)
		}
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
		req := newJSONRequest(t, "GET", "/api/v1/admin/users/", nil)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var list []any
		if err := json.Unmarshal(body, &list); err != nil {
			t.Fatalf("failed to decode JSON array response: %v", err)
		}
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
		req := newJSONRequest(t, "GET", "/api/v1/admin/users/", nil)
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusInternalServerError)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", result["error"])
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
		req := newJSONRequest(t, "POST", "/api/v1/admin/users/", map[string]string{
			"username":     "alice",
			"password":     "strongpass",
			"display_name": "Alice",
		})
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusCreated)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["username"] != "alice" {
			t.Errorf("Expected username 'alice', got '%s'", result["username"])
		}
		if result["display_name"] != "Alice" {
			t.Errorf("Expected display_name 'Alice', got '%s'", result["display_name"])
		}
	})

	t.Run("MissingUsername", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{}
		app := setupUsersApp(t, store)
		req := newJSONRequest(t, "POST", "/api/v1/admin/users/", map[string]string{
			"password": "strongpass",
		})
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusBadRequest)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "invalid_request" {
			t.Errorf("Expected error 'invalid_request', got '%s'", result["error"])
		}
	})

	t.Run("MissingPassword", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{}
		app := setupUsersApp(t, store)
		req := newJSONRequest(t, "POST", "/api/v1/admin/users/", map[string]string{
			"username": "alice",
		})
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusBadRequest)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "invalid_request" {
			t.Errorf("Expected error 'invalid_request', got '%s'", result["error"])
		}
	})

	t.Run("EmptyBody", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{}
		app := setupUsersApp(t, store)
		req := newJSONRequest(t, "POST", "/api/v1/admin/users/", map[string]string{})
		resp, bodyBytes := doRequest(t, app, req)
		assertStatus(t, resp, bodyBytes, http.StatusBadRequest)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{}
		app := setupUsersApp(t, store)

		req := httptest.NewRequest("POST", "/api/v1/admin/users/", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)
		assertStatus(t, resp, bodyBytes, http.StatusBadRequest)
	})

	t.Run("ConflictAlreadyExists", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			CreateFunc: func(_, _, _ string) (*model.User, error) {
				return nil, model.AlreadyExistsError("user already exists")
			},
		}
		app := setupUsersApp(t, store)
		req := newJSONRequest(t, "POST", "/api/v1/admin/users/", map[string]string{
			"username": "alice",
			"password": "strongpass",
		})
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusConflict)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "invalid_request" {
			t.Errorf("Expected error 'invalid_request', got '%s'", result["error"])
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
		req := newJSONRequest(t, "POST", "/api/v1/admin/users/", map[string]string{
			"username": "alice",
			"password": "strongpass",
		})
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusInternalServerError)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", result["error"])
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
		req := newJSONRequest(t, "GET", "/api/v1/admin/users/alice", nil)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["username"] != "alice" {
			t.Errorf("Expected username 'alice', got '%s'", result["username"])
		}
		if result["display_name"] != "Alice" {
			t.Errorf("Expected display_name 'Alice', got '%s'", result["display_name"])
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
		req := newJSONRequest(t, "GET", "/api/v1/admin/users/unknown", nil)
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusNotFound)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "not_found" {
			t.Errorf("Expected error 'not_found', got '%s'", result["error"])
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
		req := newJSONRequest(t, "GET", "/api/v1/admin/users/alice", nil)
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusInternalServerError)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", result["error"])
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
		req := newJSONRequest(t, "PUT", "/api/v1/admin/users/alice", map[string]string{
			"display_name": "Alice Updated",
		})
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["display_name"] != "Alice Updated" {
			t.Errorf("Expected display_name 'Alice Updated', got '%s'", result["display_name"])
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
		req := newJSONRequest(t, "PUT", "/api/v1/admin/users/alice", map[string]string{
			"password": "newpass123",
		})
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusOK)
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
		req := newJSONRequest(t, "PUT", "/api/v1/admin/users/alice", map[string]any{
			"disabled": true,
		})
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["disabled"] != true {
			t.Errorf("Expected disabled true, got %v", result["disabled"])
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
		req := newJSONRequest(t, "PUT", "/api/v1/admin/users/unknown", map[string]string{
			"display_name": "Test",
		})
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusNotFound)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "not_found" {
			t.Errorf("Expected error 'not_found', got '%s'", result["error"])
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{}
		app := setupUsersApp(t, store)

		req := httptest.NewRequest("PUT", "/api/v1/admin/users/alice", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)
		assertStatus(t, resp, bodyBytes, http.StatusBadRequest)
	})

	t.Run("InternalError", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			UpdateFunc: func(_ string, _ *string, _ *string, _ *bool) (*model.User, error) {
				return nil, fiber.ErrInternalServerError
			},
		}
		app := setupUsersApp(t, store)
		req := newJSONRequest(t, "PUT", "/api/v1/admin/users/alice", map[string]string{
			"display_name": "Test",
		})
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusInternalServerError)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", result["error"])
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
		req := newJSONRequest(t, "DELETE", "/api/v1/admin/users/alice", nil)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusNoContent)

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
		req := newJSONRequest(t, "DELETE", "/api/v1/admin/users/unknown", nil)
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusNotFound)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "not_found" {
			t.Errorf("Expected error 'not_found', got '%s'", result["error"])
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
		req := newJSONRequest(t, "DELETE", "/api/v1/admin/users/alice", nil)
		resp, body := doRequest(t, app, req)
		assertStatus(t, resp, body, http.StatusInternalServerError)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}
		if result["error"] != "server_error" {
			t.Errorf("Expected error 'server_error', got '%s'", result["error"])
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
		req := newJSONRequest(t, "GET", "/api/v1/admin/users/", nil)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusOK)
	})

	t.Run("WithUsers_RequiresAuth", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
		}
		app := newAppWithAuth(store)
		req := newJSONRequest(t, "GET", "/api/v1/admin/users/", nil)
		resp, bodyBytes := doRequest(t, app, req)
		assertStatus(t, resp, bodyBytes, http.StatusUnauthorized)
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
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusOK)
	})

	t.Run("WithUsers_CreateRequiresAuth", func(t *testing.T) {
		t.Parallel()
		store := &mockUsersStore{
			CountFunc: func() (int64, error) {
				return 1, nil
			},
		}
		app := newAppWithAuth(store)
		req := newJSONRequest(t, "POST", "/api/v1/admin/users/", map[string]string{
			"username": "newuser",
			"password": "pass",
		})
		resp, bodyBytes := doRequest(t, app, req)
		assertStatus(t, resp, bodyBytes, http.StatusUnauthorized)
	})
}
