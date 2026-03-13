package adminapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// --- MOCKS ---

type mockTrustMarkOwnersStore struct {
	model.TrustMarkOwnersStore
	listFn       func() ([]model.TrustMarkOwner, error)
	createFn     func(owner model.AddTrustMarkOwner) (*model.TrustMarkOwner, error)
	getFn        func(id string) (*model.TrustMarkOwner, error)
	updateFn     func(id string, owner model.AddTrustMarkOwner) (*model.TrustMarkOwner, error)
	deleteFn     func(id string) error
	typesFn      func(id string) ([]uint, error)
	setTypesFn   func(id string, typeIdents []string) ([]uint, error)
	addTypeFn    func(id string, typeID uint) ([]uint, error)
	deleteTypeFn func(id string, typeID uint) ([]uint, error)
}

func (m *mockTrustMarkOwnersStore) List() ([]model.TrustMarkOwner, error) {
	if m.listFn != nil {
		return m.listFn()
	}
	return nil, nil
}
func (m *mockTrustMarkOwnersStore) Create(owner model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
	if m.createFn != nil {
		return m.createFn(owner)
	}
	return &model.TrustMarkOwner{EntityID: owner.EntityID}, nil
}
func (m *mockTrustMarkOwnersStore) Get(id string) (*model.TrustMarkOwner, error) {
	if m.getFn != nil {
		return m.getFn(id)
	}
	return nil, nil
}
func (m *mockTrustMarkOwnersStore) Update(id string, owner model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
	if m.updateFn != nil {
		return m.updateFn(id, owner)
	}
	return &model.TrustMarkOwner{EntityID: owner.EntityID}, nil
}
func (m *mockTrustMarkOwnersStore) Delete(id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(id)
	}
	return nil
}
func (m *mockTrustMarkOwnersStore) Types(id string) ([]uint, error) {
	if m.typesFn != nil {
		return m.typesFn(id)
	}
	return nil, nil
}
func (m *mockTrustMarkOwnersStore) SetTypes(id string, typeIdents []string) ([]uint, error) {
	if m.setTypesFn != nil {
		return m.setTypesFn(id, typeIdents)
	}
	return nil, nil
}
func (m *mockTrustMarkOwnersStore) AddType(id string, typeID uint) ([]uint, error) {
	if m.addTypeFn != nil {
		return m.addTypeFn(id, typeID)
	}
	return nil, nil
}
func (m *mockTrustMarkOwnersStore) DeleteType(id string, typeID uint) ([]uint, error) {
	if m.deleteTypeFn != nil {
		return m.deleteTypeFn(id, typeID)
	}
	return nil, nil
}

type mockTrustMarkTypesStoreForOwners struct {
	model.TrustMarkTypesStore
	getFn func(id string) (*model.TrustMarkType, error)
}

func (m *mockTrustMarkTypesStoreForOwners) Get(id string) (*model.TrustMarkType, error) {
	if m.getFn != nil {
		return m.getFn(id)
	}
	return &model.TrustMarkType{ID: 1, TrustMarkType: "type-" + id}, nil
}

type mockTrustMarkIssuersStore struct {
	model.TrustMarkIssuersStore
	listFn       func() ([]model.TrustMarkIssuer, error)
	createFn     func(issuer model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error)
	getFn        func(id string) (*model.TrustMarkIssuer, error)
	updateFn     func(id string, issuer model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error)
	deleteFn     func(id string) error
	typesFn      func(id string) ([]uint, error)
	setTypesFn   func(id string, typeIdents []string) ([]uint, error)
	addTypeFn    func(id string, typeID uint) ([]uint, error)
	deleteTypeFn func(id string, typeID uint) ([]uint, error)
}

func (m *mockTrustMarkIssuersStore) List() ([]model.TrustMarkIssuer, error) {
	if m.listFn != nil {
		return m.listFn()
	}
	return nil, nil
}
func (m *mockTrustMarkIssuersStore) Create(issuer model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error) {
	if m.createFn != nil {
		return m.createFn(issuer)
	}
	return &model.TrustMarkIssuer{Issuer: issuer.Issuer}, nil
}
func (m *mockTrustMarkIssuersStore) Get(id string) (*model.TrustMarkIssuer, error) {
	if m.getFn != nil {
		return m.getFn(id)
	}
	return nil, nil
}
func (m *mockTrustMarkIssuersStore) Update(id string, issuer model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error) {
	if m.updateFn != nil {
		return m.updateFn(id, issuer)
	}
	return &model.TrustMarkIssuer{Issuer: issuer.Issuer}, nil
}
func (m *mockTrustMarkIssuersStore) Delete(id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(id)
	}
	return nil
}
func (m *mockTrustMarkIssuersStore) Types(id string) ([]uint, error) {
	if m.typesFn != nil {
		return m.typesFn(id)
	}
	return nil, nil
}
func (m *mockTrustMarkIssuersStore) SetTypes(id string, typeIdents []string) ([]uint, error) {
	if m.setTypesFn != nil {
		return m.setTypesFn(id, typeIdents)
	}
	return nil, nil
}
func (m *mockTrustMarkIssuersStore) AddType(id string, typeID uint) ([]uint, error) {
	if m.addTypeFn != nil {
		return m.addTypeFn(id, typeID)
	}
	return nil, nil
}
func (m *mockTrustMarkIssuersStore) DeleteType(id string, typeID uint) ([]uint, error) {
	if m.deleteTypeFn != nil {
		return m.deleteTypeFn(id, typeID)
	}
	return nil, nil
}

// --- SETUP HELPERS ---

func setupTrustMarkOwnersApp(ownersStore model.TrustMarkOwnersStore, typesStore model.TrustMarkTypesStore) *fiber.App {
	app := fiber.New()
	registerTrustMarkOwners(app, ownersStore, typesStore)
	return app
}

func setupTrustMarkIssuersApp(issuersStore model.TrustMarkIssuersStore, typesStore model.TrustMarkTypesStore) *fiber.App {
	app := fiber.New()
	registerTrustMarkIssuers(app, issuersStore, typesStore)
	return app
}

// --- TESTS ---

func TestTrustMarkOwnersHandlers_List(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			listFn: func() ([]model.TrustMarkOwner, error) {
				return []model.TrustMarkOwner{{EntityID: "owner1"}}, nil
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/owners", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "owner1") {
			t.Errorf("Expected response to contain 'owner1'")
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			listFn: func() ([]model.TrustMarkOwner, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/owners", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkOwnersHandlers_Create(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		body := `{"entity_id": "owner1"}`
		req := httptest.NewRequest("POST", "/trust-marks/owners", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusCreated)
		if !strings.Contains(string(respBody), "owner1") {
			t.Errorf("Expected response to contain 'owner1'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkOwnersApp(&mockTrustMarkOwnersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/owners", strings.NewReader(`invalid json`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("OwnerIDNotAllowed", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkOwnersApp(&mockTrustMarkOwnersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/owners", strings.NewReader(`{"owner_id": 123, "entity_id": "owner1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("MissingEntityID", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkOwnersApp(&mockTrustMarkOwnersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/owners", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			createFn: func(owner model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
				return nil, model.AlreadyExistsError("exists")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/owners", strings.NewReader(`{"entity_id": "owner1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusConflict)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			createFn: func(owner model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/owners", strings.NewReader(`{"entity_id": "owner1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkOwnersHandlers_Get(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			getFn: func(id string) (*model.TrustMarkOwner, error) {
				return &model.TrustMarkOwner{EntityID: "owner1"}, nil
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/owners/1", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "owner1") {
			t.Errorf("Expected response to contain 'owner1'")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			getFn: func(id string) (*model.TrustMarkOwner, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/owners/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})
}

func TestTrustMarkOwnersHandlers_Update(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		body := `{"entity_id": "owner2"}`
		req := httptest.NewRequest("PUT", "/trust-marks/owners/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(respBody), "owner2") {
			t.Errorf("Expected response to contain 'owner2'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkOwnersApp(&mockTrustMarkOwnersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/owners/1", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			updateFn: func(id string, owner model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/owners/1", strings.NewReader(`{"entity_id": "owner2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			updateFn: func(id string, owner model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
				return nil, model.AlreadyExistsError("exists")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/owners/1", strings.NewReader(`{"entity_id": "owner2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusConflict)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			updateFn: func(id string, owner model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/owners/1", strings.NewReader(`{"entity_id": "owner2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkOwnersHandlers_Delete(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("DELETE", "/trust-marks/owners/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNoContent)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			deleteFn: func(id string) error {
				return model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("DELETE", "/trust-marks/owners/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})
}

func TestTrustMarkOwnersHandlers_TypesList(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			typesFn: func(id string) ([]uint, error) {
				return []uint{1, 2}, nil
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/owners/1/types", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "type-1") || !strings.Contains(string(body), "type-2") {
			t.Errorf("Expected response to contain 'type-1' and 'type-2'")
		}
	})

	t.Run("OwnerNotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			typesFn: func(id string) ([]uint, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/owners/1/types", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("LoadTypesError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			typesFn: func(id string) ([]uint, error) {
				return []uint{1, 2}, nil
			},
		}
		mockTypesStore := &mockTrustMarkTypesStoreForOwners{
			getFn: func(id string) (*model.TrustMarkType, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, mockTypesStore)

		req := httptest.NewRequest("GET", "/trust-marks/owners/1/types", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkOwnersHandlers_TypesSet(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			setTypesFn: func(id string, typeIdents []string) ([]uint, error) {
				return []uint{3}, nil
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/owners/1/types", strings.NewReader(`["typeA"]`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "type-3") {
			t.Errorf("Expected response to contain 'type-3'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkOwnersApp(&mockTrustMarkOwnersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/owners/1/types", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("OwnerNotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			setTypesFn: func(id string, typeIdents []string) ([]uint, error) {
				return nil, errors.New("db error") // Generic error since SetTypes doesn't explicitly return NotFound yet
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/owners/1/types", strings.NewReader(`["typeA"]`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})

	t.Run("LoadTypesError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			setTypesFn: func(id string, typeIdents []string) ([]uint, error) {
				return []uint{1}, nil
			},
		}
		mockTypesStore := &mockTrustMarkTypesStoreForOwners{
			getFn: func(id string) (*model.TrustMarkType, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, mockTypesStore)

		req := httptest.NewRequest("PUT", "/trust-marks/owners/1/types", strings.NewReader(`["typeA"]`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkOwnersHandlers_TypesAdd(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			addTypeFn: func(id string, typeID uint) ([]uint, error) {
				return []uint{4}, nil
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/owners/1/types", strings.NewReader("4"))
		req.Header.Set("Content-Type", "text/plain")
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusCreated)
		if !strings.Contains(string(body), "type-4") {
			t.Errorf("Expected response to contain 'type-4'")
		}
	})

	t.Run("InvalidBodyNotInt", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkOwnersApp(&mockTrustMarkOwnersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/owners/1/types", strings.NewReader("not_an_int"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("OwnerNotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			addTypeFn: func(id string, typeID uint) ([]uint, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/owners/1/types", strings.NewReader("4"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})

	t.Run("LoadTypesError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			addTypeFn: func(id string, typeID uint) ([]uint, error) {
				return []uint{4}, nil
			},
		}
		mockTypesStore := &mockTrustMarkTypesStoreForOwners{
			getFn: func(id string) (*model.TrustMarkType, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, mockTypesStore)

		req := httptest.NewRequest("POST", "/trust-marks/owners/1/types", strings.NewReader("4"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkOwnersHandlers_TypesDelete(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			deleteTypeFn: func(id string, typeID uint) ([]uint, error) {
				return []uint{}, nil
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("DELETE", "/trust-marks/owners/1/types/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("TypeNotFound", func(t *testing.T) {
		t.Parallel()
		mockTypesStore := &mockTrustMarkTypesStoreForOwners{
			getFn: func(id string) (*model.TrustMarkType, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkOwnersApp(&mockTrustMarkOwnersStore{}, mockTypesStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/owners/1/types/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("DeleteError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkOwnersStore{
			deleteTypeFn: func(id string, typeID uint) ([]uint, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkOwnersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("DELETE", "/trust-marks/owners/1/types/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestGlobalTrustMarkIssuersHandlers_List(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			listFn: func() ([]model.TrustMarkIssuer, error) {
				return []model.TrustMarkIssuer{{Issuer: "issuer1"}}, nil
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/issuers", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "issuer1") {
			t.Errorf("Expected response to contain 'issuer1'")
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			listFn: func() ([]model.TrustMarkIssuer, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/issuers", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestGlobalTrustMarkIssuersHandlers_Create(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		body := `{"issuer": "issuer1"}`
		req := httptest.NewRequest("POST", "/trust-marks/issuers", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusCreated)
		if !strings.Contains(string(respBody), "issuer1") {
			t.Errorf("Expected response to contain 'issuer1'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuersApp(&mockTrustMarkIssuersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/issuers", strings.NewReader(`invalid json`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("IssuerIDNotAllowed", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuersApp(&mockTrustMarkIssuersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/issuers", strings.NewReader(`{"issuer_id": 123, "issuer": "issuer1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("MissingIssuer", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuersApp(&mockTrustMarkIssuersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/issuers", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			createFn: func(issuer model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error) {
				return nil, model.AlreadyExistsError("exists")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/issuers", strings.NewReader(`{"issuer": "issuer1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusConflict)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			createFn: func(issuer model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/issuers", strings.NewReader(`{"issuer": "issuer1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestGlobalTrustMarkIssuersHandlers_Get(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			getFn: func(id string) (*model.TrustMarkIssuer, error) {
				return &model.TrustMarkIssuer{Issuer: "issuer1"}, nil
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/issuers/1", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "issuer1") {
			t.Errorf("Expected response to contain 'issuer1'")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			getFn: func(id string) (*model.TrustMarkIssuer, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/issuers/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})
}

func TestGlobalTrustMarkIssuersHandlers_Update(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		body := `{"issuer": "issuer2"}`
		req := httptest.NewRequest("PUT", "/trust-marks/issuers/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(respBody), "issuer2") {
			t.Errorf("Expected response to contain 'issuer2'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuersApp(&mockTrustMarkIssuersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuers/1", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			updateFn: func(id string, issuer model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuers/1", strings.NewReader(`{"issuer": "issuer2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			updateFn: func(id string, issuer model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error) {
				return nil, model.AlreadyExistsError("exists")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuers/1", strings.NewReader(`{"issuer": "issuer2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusConflict)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			updateFn: func(id string, issuer model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuers/1", strings.NewReader(`{"issuer": "issuer2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestGlobalTrustMarkIssuersHandlers_Delete(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("DELETE", "/trust-marks/issuers/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNoContent)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			deleteFn: func(id string) error {
				return model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("DELETE", "/trust-marks/issuers/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})
}

func TestGlobalTrustMarkIssuersHandlers_TypesList(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			typesFn: func(id string) ([]uint, error) {
				return []uint{1, 2}, nil
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/issuers/1/types", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "type-1") || !strings.Contains(string(body), "type-2") {
			t.Errorf("Expected response to contain 'type-1' and 'type-2'")
		}
	})

	t.Run("IssuerNotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			typesFn: func(id string) ([]uint, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("GET", "/trust-marks/issuers/1/types", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("LoadTypesError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			typesFn: func(id string) ([]uint, error) {
				return []uint{1, 2}, nil
			},
		}
		mockTypesStore := &mockTrustMarkTypesStoreForOwners{
			getFn: func(id string) (*model.TrustMarkType, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, mockTypesStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuers/1/types", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestGlobalTrustMarkIssuersHandlers_TypesSet(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			setTypesFn: func(id string, typeIdents []string) ([]uint, error) {
				return []uint{3}, nil
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuers/1/types", strings.NewReader(`["typeA"]`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "type-3") {
			t.Errorf("Expected response to contain 'type-3'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuersApp(&mockTrustMarkIssuersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuers/1/types", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("IssuerNotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			setTypesFn: func(id string, typeIdents []string) ([]uint, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuers/1/types", strings.NewReader(`["typeA"]`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})

	t.Run("LoadTypesError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			setTypesFn: func(id string, typeIdents []string) ([]uint, error) {
				return []uint{1}, nil
			},
		}
		mockTypesStore := &mockTrustMarkTypesStoreForOwners{
			getFn: func(id string) (*model.TrustMarkType, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, mockTypesStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuers/1/types", strings.NewReader(`["typeA"]`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestGlobalTrustMarkIssuersHandlers_TypesAdd(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			addTypeFn: func(id string, typeID uint) ([]uint, error) {
				return []uint{4}, nil
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/issuers/1/types", strings.NewReader("4"))
		req.Header.Set("Content-Type", "text/plain")
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusCreated)
		if !strings.Contains(string(body), "type-4") {
			t.Errorf("Expected response to contain 'type-4'")
		}
	})

	t.Run("InvalidBodyNotInt", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuersApp(&mockTrustMarkIssuersStore{}, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/issuers/1/types", strings.NewReader("not_an_int"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("IssuerNotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			addTypeFn: func(id string, typeID uint) ([]uint, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("POST", "/trust-marks/issuers/1/types", strings.NewReader("4"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})

	t.Run("LoadTypesError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			addTypeFn: func(id string, typeID uint) ([]uint, error) {
				return []uint{4}, nil
			},
		}
		mockTypesStore := &mockTrustMarkTypesStoreForOwners{
			getFn: func(id string) (*model.TrustMarkType, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, mockTypesStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuers/1/types", strings.NewReader("4"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestGlobalTrustMarkIssuersHandlers_TypesDelete(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			deleteTypeFn: func(id string, typeID uint) ([]uint, error) {
				return []uint{}, nil
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("DELETE", "/trust-marks/issuers/1/types/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("TypeNotFound", func(t *testing.T) {
		t.Parallel()
		mockTypesStore := &mockTrustMarkTypesStoreForOwners{
			getFn: func(id string) (*model.TrustMarkType, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuersApp(&mockTrustMarkIssuersStore{}, mockTypesStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuers/1/types/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("DeleteError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkIssuersStore{
			deleteTypeFn: func(id string, typeID uint) ([]uint, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuersApp(mockStore, &mockTrustMarkTypesStoreForOwners{})

		req := httptest.NewRequest("DELETE", "/trust-marks/issuers/1/types/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}
