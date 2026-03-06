package adminapi

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// --- MOCKS ---

type mockTrustMarkSpecStore struct {
	model.TrustMarkSpecStore
	listFn                 func() ([]model.TrustMarkSpec, error)
	createFn               func(spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error)
	getFn                  func(id string) (*model.TrustMarkSpec, error)
	updateFn               func(id string, spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error)
	patchFn                func(id string, updates map[string]any) (*model.TrustMarkSpec, error)
	deleteFn               func(id string) error

	listSubjectsFn        func(specID string, status *model.Status) ([]model.TrustMarkSubject, error)
	createSubjectFn       func(specID string, subject *model.TrustMarkSubject) (*model.TrustMarkSubject, error)
	getSubjectFn          func(specID, subjectID string) (*model.TrustMarkSubject, error)
	updateSubjectFn       func(specID, subjectID string, subject *model.TrustMarkSubject) (*model.TrustMarkSubject, error)
	deleteSubjectFn       func(specID, subjectID string) error
	changeSubjectStatusFn func(specID, subjectID string, status model.Status) (*model.TrustMarkSubject, error)
}

func (m *mockTrustMarkSpecStore) List() ([]model.TrustMarkSpec, error) {
	if m.listFn != nil {
		return m.listFn()
	}
	return nil, nil
}
func (m *mockTrustMarkSpecStore) Create(spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
	if m.createFn != nil {
		return m.createFn(spec)
	}
	return spec, nil
}
func (m *mockTrustMarkSpecStore) Get(id string) (*model.TrustMarkSpec, error) {
	if m.getFn != nil {
		return m.getFn(id)
	}
	return nil, nil
}
func (m *mockTrustMarkSpecStore) Update(id string, spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
	if m.updateFn != nil {
		return m.updateFn(id, spec)
	}
	return spec, nil
}
func (m *mockTrustMarkSpecStore) Patch(id string, updates map[string]any) (*model.TrustMarkSpec, error) {
	if m.patchFn != nil {
		return m.patchFn(id, updates)
	}
	return nil, nil
}
func (m *mockTrustMarkSpecStore) Delete(id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(id)
	}
	return nil
}

func (m *mockTrustMarkSpecStore) ListSubjects(specID string, status *model.Status) ([]model.TrustMarkSubject, error) {
	if m.listSubjectsFn != nil {
		return m.listSubjectsFn(specID, status)
	}
	return nil, nil
}
func (m *mockTrustMarkSpecStore) CreateSubject(specID string, subject *model.TrustMarkSubject) (*model.TrustMarkSubject, error) {
	if m.createSubjectFn != nil {
		return m.createSubjectFn(specID, subject)
	}
	return subject, nil
}
func (m *mockTrustMarkSpecStore) GetSubject(specID, subjectID string) (*model.TrustMarkSubject, error) {
	if m.getSubjectFn != nil {
		return m.getSubjectFn(specID, subjectID)
	}
	return nil, nil
}
func (m *mockTrustMarkSpecStore) UpdateSubject(specID, subjectID string, subject *model.TrustMarkSubject) (*model.TrustMarkSubject, error) {
	if m.updateSubjectFn != nil {
		return m.updateSubjectFn(specID, subjectID, subject)
	}
	return subject, nil
}
func (m *mockTrustMarkSpecStore) DeleteSubject(specID, subjectID string) error {
	if m.deleteSubjectFn != nil {
		return m.deleteSubjectFn(specID, subjectID)
	}
	return nil
}
func (m *mockTrustMarkSpecStore) ChangeSubjectStatus(specID, subjectID string, status model.Status) (*model.TrustMarkSubject, error) {
	if m.changeSubjectStatusFn != nil {
		return m.changeSubjectStatusFn(specID, subjectID, status)
	}
	return nil, nil
}

// --- SETUP HELPERS ---

func setupTrustMarkIssuanceApp(store model.TrustMarkSpecStore) *fiber.App {
	app := fiber.New()
	registerTrustMarkIssuance(app, store)
	return app
}

func doTrustMarkReq(t *testing.T, app *fiber.App, req *http.Request) (*http.Response, []byte) {
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

func TestTrustMarkSpecHandlers_List(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			listFn: func() ([]model.TrustMarkSpec, error) {
				return []model.TrustMarkSpec{{TrustMarkType: "type1"}}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec", http.NoBody)
		resp, body := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "type1") {
			t.Errorf("Expected response to contain 'type1', got %s", string(body))
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			listFn: func() ([]model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec", http.NoBody)
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkSpecHandlers_Create(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			createFn: func(spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return spec, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		body := `{"trust_mark_type": "type1"}`
		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(respBody), "type1") {
			t.Errorf("Expected response to contain 'type1', got %s", string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`invalid json`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("MissingType", func(t *testing.T) {
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			createFn: func(spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, model.AlreadyExistsError("exists")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`{"trust_mark_type": "type1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected 409, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			createFn: func(spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`{"trust_mark_type": "type1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkSpecHandlers_Get(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(id string) (*model.TrustMarkSpec, error) {
				return &model.TrustMarkSpec{TrustMarkType: "type1"}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, body := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "type1") {
			t.Errorf("Expected response to contain 'type1', got %s", string(body))
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(id string) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(id string) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkSpecHandlers_Update(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			updateFn: func(id string, spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return spec, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		body := `{"trust_mark_type": "type2"}`
		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(respBody), "type2") {
			t.Errorf("Expected response to contain 'type2', got %s", string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("MissingType", func(t *testing.T) {
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			updateFn: func(id string, spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			updateFn: func(id string, spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkSpecHandlers_Patch(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			patchFn: func(id string, updates map[string]any) (*model.TrustMarkSpec, error) {
				return &model.TrustMarkSpec{TrustMarkType: "type3"}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		body := `{"trust_mark_type": "type3"}`
		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(respBody), "type3") {
			t.Errorf("Expected response to contain 'type3', got %s", string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			patchFn: func(id string, updates map[string]any) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type3"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			patchFn: func(id string, updates map[string]any) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type3"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}

func TestTrustMarkSpecHandlers_Delete(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			deleteFn: func(id string) error {
				return nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected 204, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			deleteFn: func(id string) error {
				return model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore := &mockTrustMarkSpecStore{
			deleteFn: func(id string) error {
				return errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, _ := doTrustMarkReq(t, app, req)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", resp.StatusCode)
		}
	})
}
