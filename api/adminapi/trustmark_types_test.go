package adminapi

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// --- MOCKS ---

type mockTrustMarkTypesStore struct {
	model.TrustMarkTypesStore
	listFn              func() ([]model.TrustMarkType, error)
	createFn            func(req model.AddTrustMarkType) (*model.TrustMarkType, error)
	getFn               func(id string) (*model.TrustMarkType, error)
	updateFn            func(id string, req model.AddTrustMarkType) (*model.TrustMarkType, error)
	deleteFn            func(id string) error
	listIssuersFn       func(id string) ([]model.TrustMarkIssuer, error)
	setIssuersFn        func(id string, issuers []model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error)
	addIssuerFn         func(id string, issuer model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error)
	deleteIssuerByIDFn  func(id string, issuerID uint) ([]model.TrustMarkIssuer, error)
	deleteIssuerByStringFn func(id string, issuer string) ([]model.TrustMarkIssuer, error)
	getOwnerFn          func(id string) (*model.TrustMarkOwner, error)
	createOwnerFn       func(id string, req model.AddTrustMarkOwner) (*model.TrustMarkOwner, error)
	updateOwnerFn       func(id string, req model.AddTrustMarkOwner) (*model.TrustMarkOwner, error)
	deleteOwnerFn       func(id string) error
	
	ownersByTypeFn func() (oidfed.TrustMarkOwners, error)
	issuersByTypeFn func() (oidfed.AllowedTrustMarkIssuers, error)
}

func (m *mockTrustMarkTypesStore) List() ([]model.TrustMarkType, error) {
	if m.listFn != nil {
		return m.listFn()
	}
	return nil, nil
}
func (m *mockTrustMarkTypesStore) Create(req model.AddTrustMarkType) (*model.TrustMarkType, error) {
	if m.createFn != nil {
		return m.createFn(req)
	}
	return &model.TrustMarkType{TrustMarkType: req.TrustMarkType}, nil
}
func (m *mockTrustMarkTypesStore) Get(id string) (*model.TrustMarkType, error) {
	if m.getFn != nil {
		return m.getFn(id)
	}
	return nil, nil
}
func (m *mockTrustMarkTypesStore) Update(id string, req model.AddTrustMarkType) (*model.TrustMarkType, error) {
	if m.updateFn != nil {
		return m.updateFn(id, req)
	}
	return &model.TrustMarkType{TrustMarkType: "updated-" + id}, nil
}
func (m *mockTrustMarkTypesStore) Delete(id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(id)
	}
	return nil
}
func (m *mockTrustMarkTypesStore) ListIssuers(id string) ([]model.TrustMarkIssuer, error) {
	if m.listIssuersFn != nil {
		return m.listIssuersFn(id)
	}
	return nil, nil
}
func (m *mockTrustMarkTypesStore) SetIssuers(id string, issuers []model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error) {
	if m.setIssuersFn != nil {
		return m.setIssuersFn(id, issuers)
	}
	return nil, nil
}
func (m *mockTrustMarkTypesStore) AddIssuer(id string, issuer model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error) {
	if m.addIssuerFn != nil {
		return m.addIssuerFn(id, issuer)
	}
	return nil, nil
}
func (m *mockTrustMarkTypesStore) DeleteIssuerByID(id string, issuerID uint) ([]model.TrustMarkIssuer, error) {
	if m.deleteIssuerByIDFn != nil {
		return m.deleteIssuerByIDFn(id, issuerID)
	}
	return nil, nil
}
func (m *mockTrustMarkTypesStore) DeleteIssuerByString(id string, issuer string) ([]model.TrustMarkIssuer, error) {
	if m.deleteIssuerByStringFn != nil {
		return m.deleteIssuerByStringFn(id, issuer)
	}
	return nil, nil
}
func (m *mockTrustMarkTypesStore) GetOwner(id string) (*model.TrustMarkOwner, error) {
	if m.getOwnerFn != nil {
		return m.getOwnerFn(id)
	}
	return nil, nil
}
func (m *mockTrustMarkTypesStore) CreateOwner(id string, req model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
	if m.createOwnerFn != nil {
		return m.createOwnerFn(id, req)
	}
	return nil, nil
}
func (m *mockTrustMarkTypesStore) UpdateOwner(id string, req model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
	if m.updateOwnerFn != nil {
		return m.updateOwnerFn(id, req)
	}
	return nil, nil
}
func (m *mockTrustMarkTypesStore) DeleteOwner(id string) error {
	if m.deleteOwnerFn != nil {
		return m.deleteOwnerFn(id)
	}
	return nil
}

func (m *mockTrustMarkTypesStore) OwnersByType() (oidfed.TrustMarkOwners, error) {
	if m.ownersByTypeFn != nil {
		return m.ownersByTypeFn()
	}
	return nil, nil
}

func (m *mockTrustMarkTypesStore) IssuersByType() (oidfed.AllowedTrustMarkIssuers, error) {
	if m.issuersByTypeFn != nil {
		return m.issuersByTypeFn()
	}
	return nil, nil
}


// --- SETUP HELPERS ---

func setupTrustMarkTypesApp(typesStore model.TrustMarkTypesStore) *fiber.App {
	app := fiber.New()
	backends := model.Backends{
		TrustMarkTypes: typesStore,
		Transaction: func(fn model.TransactionFunc) error {
			// for tests, just run the function with the same backends
			return fn(&model.Backends{TrustMarkTypes: typesStore})
		},
	}
	registerTrustMarkTypes(app, backends)
	return app
}

func doTrustMarkTypesReq(t *testing.T, app *fiber.App, req *http.Request) (*http.Response, []byte) {
	t.Helper()
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Request %s %s failed: %v", req.Method, req.URL.Path, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	return resp, body
}

// --- TESTS ---

func TestTrustMarkTypesHandlers_List(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			listFn: func() ([]model.TrustMarkType, error) {
				return []model.TrustMarkType{{TrustMarkType: "type1"}}, nil
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/types", http.NoBody)
		resp, body := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "type1") {
			t.Errorf("Expected response to contain 'type1'")
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			listFn: func() ([]model.TrustMarkType, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/types", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkTypesHandlers_Create(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{}
		app := setupTrustMarkTypesApp(mockStore)

		body := `{"trust_mark_type": "type1"}`
		req := httptest.NewRequest("POST", "/trust-marks/types", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(respBody), "type1") {
			t.Errorf("Expected response to contain 'type1'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupTrustMarkTypesApp(&mockTrustMarkTypesStore{})

		req := httptest.NewRequest("POST", "/trust-marks/types", strings.NewReader(`invalid json`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("MissingType", func(t *testing.T) {
		app := setupTrustMarkTypesApp(&mockTrustMarkTypesStore{})

		req := httptest.NewRequest("POST", "/trust-marks/types", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			createFn: func(req model.AddTrustMarkType) (*model.TrustMarkType, error) {
				return nil, model.AlreadyExistsError("exists")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/types", strings.NewReader(`{"trust_mark_type": "type1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected 409, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			createFn: func(req model.AddTrustMarkType) (*model.TrustMarkType, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/types", strings.NewReader(`{"trust_mark_type": "type1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkTypesHandlers_Get(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			getFn: func(id string) (*model.TrustMarkType, error) {
				return &model.TrustMarkType{TrustMarkType: "type1"}, nil
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/types/1", http.NoBody)
		resp, body := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "type1") {
			t.Errorf("Expected response to contain 'type1'")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			getFn: func(id string) (*model.TrustMarkType, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/types/1", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkTypesHandlers_Update(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{}
		app := setupTrustMarkTypesApp(mockStore)

		body := `{"trust_mark_type": "type2"}`
		req := httptest.NewRequest("PUT", "/trust-marks/types/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(respBody), "updated-1") { // Default mock return
			t.Errorf("Expected response to contain 'updated-1', got %s", string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupTrustMarkTypesApp(&mockTrustMarkTypesStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/types/1", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("MissingType", func(t *testing.T) {
		app := setupTrustMarkTypesApp(&mockTrustMarkTypesStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/types/1", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("SuccessWithOwnerAndIssuers", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			updateFn: func(id string, req model.AddTrustMarkType) (*model.TrustMarkType, error) {
				return &model.TrustMarkType{TrustMarkType: "tx-updated"}, nil
			},
			updateOwnerFn: func(id string, req model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
				return &model.TrustMarkOwner{EntityID: req.EntityID}, nil
			},
			setIssuersFn: func(id string, issuers []model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error) {
				return []model.TrustMarkIssuer{}, nil
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		body := `{"trust_mark_type": "type3", "owner": {"entity_id": "owner1"}, "issuers": [{"issuer": "iss1"}]}`
		req := httptest.NewRequest("PUT", "/trust-marks/types/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(respBody), "tx-updated") {
			t.Errorf("Expected response to contain 'tx-updated'")
		}
	})

	t.Run("TxError", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			updateFn: func(id string, req model.AddTrustMarkType) (*model.TrustMarkType, error) {
				return nil, errors.New("tx error")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		body := `{"trust_mark_type": "type3", "owner": {"entity_id": "owner1"}}`
		req := httptest.NewRequest("PUT", "/trust-marks/types/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkTypesHandlers_Delete(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			deleteFn: func(id string) error {
				return nil
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/types/1", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected 204, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			deleteFn: func(id string) error {
				return model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/types/1", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			deleteFn: func(id string) error {
				return errors.New("db error")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/types/1", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkIssuersHandlers_List(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			listIssuersFn: func(id string) ([]model.TrustMarkIssuer, error) {
				return []model.TrustMarkIssuer{{Issuer: "iss1"}}, nil
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/types/1/issuers", http.NoBody)
		resp, body := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "iss1") {
			t.Errorf("Expected response to contain 'iss1'")
		}
	})

	t.Run("TypeNotFound", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			listIssuersFn: func(id string) ([]model.TrustMarkIssuer, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/types/1/issuers", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			listIssuersFn: func(id string) ([]model.TrustMarkIssuer, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/types/1/issuers", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkIssuersHandlers_Set(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			setIssuersFn: func(id string, issuers []model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error) {
				return []model.TrustMarkIssuer{{Issuer: "iss2"}}, nil
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/types/1/issuers", strings.NewReader(`[{"issuer": "iss2"}]`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "iss2") {
			t.Errorf("Expected response to contain 'iss2'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupTrustMarkTypesApp(&mockTrustMarkTypesStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/types/1/issuers", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("TypeNotFound", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			setIssuersFn: func(id string, issuers []model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/types/1/issuers", strings.NewReader(`[{"issuer": "iss2"}]`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			setIssuersFn: func(id string, issuers []model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/types/1/issuers", strings.NewReader(`[{"issuer": "iss2"}]`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkIssuersHandlers_Add(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			addIssuerFn: func(id string, issuer model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error) {
				return []model.TrustMarkIssuer{{Issuer: "iss3"}}, nil
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/types/1/issuers", strings.NewReader(`{"issuer": "iss3"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "iss3") {
			t.Errorf("Expected response to contain 'iss3'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupTrustMarkTypesApp(&mockTrustMarkTypesStore{})

		req := httptest.NewRequest("POST", "/trust-marks/types/1/issuers", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("TypeNotFound", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			addIssuerFn: func(id string, issuer model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/types/1/issuers", strings.NewReader(`{"issuer": "iss3"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			addIssuerFn: func(id string, issuer model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/types/1/issuers", strings.NewReader(`{"issuer": "iss3"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkIssuersHandlers_Delete(t *testing.T) {
	t.Run("SuccessNumericID", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			deleteIssuerByIDFn: func(id string, issuerID uint) ([]model.TrustMarkIssuer, error) {
				return []model.TrustMarkIssuer{}, nil
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/types/1/issuers/42", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("SuccessStringIssuer", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			addIssuerFn: func(id string, issuer model.AddTrustMarkIssuer) ([]model.TrustMarkIssuer, error) {
				return []model.TrustMarkIssuer{{ID: 42, Issuer: "notanumber"}}, nil
			},
			listIssuersFn: func(id string) ([]model.TrustMarkIssuer, error) {
				return []model.TrustMarkIssuer{{ID: 42, Issuer: "notanumber"}}, nil
			},
			deleteIssuerByIDFn: func(id string, issuerID uint) ([]model.TrustMarkIssuer, error) {
				return []model.TrustMarkIssuer{}, nil
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/types/1/issuers/notanumber", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("IssuerNotFound", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			deleteIssuerByIDFn: func(id string, issuerID uint) ([]model.TrustMarkIssuer, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/types/1/issuers/42", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteError", func(t *testing.T) {
		mockStore := &mockTrustMarkTypesStore{
			deleteIssuerByIDFn: func(id string, issuerID uint) ([]model.TrustMarkIssuer, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkTypesApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/types/1/issuers/42", http.NoBody)
		resp, _ := doTrustMarkTypesReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}
