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

type mockTrustMarkSpecStore struct {
	model.TrustMarkSpecStore
	listFn   func() ([]model.TrustMarkSpec, error)
	createFn func(spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error)
	getFn    func(id string) (*model.TrustMarkSpec, error)
	updateFn func(id string, spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error)
	patchFn  func(id string, updates map[string]any) (*model.TrustMarkSpec, error)
	deleteFn func(id string) error

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

// --- TESTS ---

func TestTrustMarkSpecHandlers_List(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			listFn: func() ([]model.TrustMarkSpec, error) {
				return []model.TrustMarkSpec{{TrustMarkType: "type1"}}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "type1") {
			t.Errorf("Expected response to contain 'type1', got %s", string(body))
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			listFn: func() ([]model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSpecHandlers_Create(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			createFn: func(spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return spec, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		body := `{"trust_mark_type": "type1"}`
		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusCreated)
		if !strings.Contains(string(respBody), "type1") {
			t.Errorf("Expected response to contain 'type1', got %s", string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`invalid json`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("MissingType", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			createFn: func(_ *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, model.AlreadyExistsError("exists")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`{"trust_mark_type": "type1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusConflict)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			createFn: func(_ *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`{"trust_mark_type": "type1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSpecHandlers_Get(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(_ string) (*model.TrustMarkSpec, error) {
				return &model.TrustMarkSpec{TrustMarkType: "type1"}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "type1") {
			t.Errorf("Expected response to contain 'type1', got %s", string(body))
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(_ string) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(_ string) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSpecHandlers_Update(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateFn: func(_ string, spec *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return spec, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		body := `{"trust_mark_type": "type2"}`
		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(respBody), "type2") {
			t.Errorf("Expected response to contain 'type2', got %s", string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("MissingType", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateFn: func(_ string, _ *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateFn: func(_ string, _ *model.TrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSpecHandlers_Patch(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			patchFn: func(_ string, _ map[string]any) (*model.TrustMarkSpec, error) {
				return &model.TrustMarkSpec{TrustMarkType: "type3"}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		body := `{"trust_mark_type": "type3"}`
		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(respBody), "type3") {
			t.Errorf("Expected response to contain 'type3', got %s", string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			patchFn: func(_ string, _ map[string]any) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type3"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			patchFn: func(_ string, _ map[string]any) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type3"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSpecHandlers_Delete(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			deleteFn: func(_ string) error {
				return nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNoContent)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			deleteFn: func(_ string) error {
				return model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			deleteFn: func(_ string) error {
				return errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSubjectHandlers_List(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			listSubjectsFn: func(_ string, _ *model.Status) ([]model.TrustMarkSubject, error) {
				return []model.TrustMarkSubject{{EntityID: "sub1"}}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "sub1") {
			t.Errorf("Expected response to contain 'sub1'")
		}
	})

	t.Run("InvalidStatusFilter", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects?status=invalid", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			listSubjectsFn: func(_ string, _ *model.Status) ([]model.TrustMarkSubject, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSubjectHandlers_Create(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			createSubjectFn: func(_ string, subject *model.TrustMarkSubject) (*model.TrustMarkSubject, error) {
				return subject, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		body := `{"entity_id": "sub1"}`
		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusCreated)
		if !strings.Contains(string(respBody), "sub1") {
			t.Errorf("Expected response to contain 'sub1'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("MissingEntityID", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			createSubjectFn: func(_ string, _ *model.TrustMarkSubject) (*model.TrustMarkSubject, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects", strings.NewReader(`{"entity_id": "sub1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSubjectHandlers_Get(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{EntityID: "sub1"}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "sub1") {
			t.Errorf("Expected response to contain 'sub1'")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSubjectHandlers_Update(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateSubjectFn: func(_, _ string, subject *model.TrustMarkSubject) (*model.TrustMarkSubject, error) {
				return subject, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		body := `{"entity_id": "sub2"}`
		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(respBody), "sub2") {
			t.Errorf("Expected response to contain 'sub2'")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("MissingEntityID", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateSubjectFn: func(_, _ string, _ *model.TrustMarkSubject) (*model.TrustMarkSubject, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2", strings.NewReader(`{"entity_id": "sub2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateSubjectFn: func(_, _ string, _ *model.TrustMarkSubject) (*model.TrustMarkSubject, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2", strings.NewReader(`{"entity_id": "sub2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSubjectHandlers_Delete(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			deleteSubjectFn: func(_, _ string) error {
				return nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNoContent)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			deleteSubjectFn: func(_, _ string) error {
				return model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			deleteSubjectFn: func(_, _ string) error {
				return errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSubjectHandlers_UpdateStatus(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			changeSubjectStatusFn: func(_, _ string, status model.Status) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{Status: status}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/status", strings.NewReader("inactive"))
		req.Header.Set("Content-Type", "text/plain")
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "inactive") {
			t.Errorf("Expected response to contain 'inactive'")
		}
	})

	t.Run("MissingStatus", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/status", strings.NewReader("   "))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("InvalidStatus", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/status", strings.NewReader("unknown"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			changeSubjectStatusFn: func(_, _ string, _ model.Status) (*model.TrustMarkSubject, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/status", strings.NewReader("active"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			changeSubjectStatusFn: func(_, _ string, _ model.Status) (*model.TrustMarkSubject, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/status", strings.NewReader("active"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}

func TestTrustMarkSubjectHandlers_AdditionalClaims(t *testing.T) {
	t.Parallel()
	t.Run("GetAdditionalClaims_SuccessWithClaims", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{AdditionalClaims: map[string]any{"claim1": "val1"}}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(body), "claim1") {
			t.Errorf("Expected response to contain 'claim1'")
		}
	})

	t.Run("GetAdditionalClaims_SuccessEmptyClaims", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{AdditionalClaims: nil}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if string(body) != "{}" {
			t.Errorf("Expected empty object, got %s", string(body))
		}
	})

	t.Run("GetAdditionalClaims_NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("PutAdditionalClaims_Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{}, nil
			},
			updateSubjectFn: func(_, _ string, subject *model.TrustMarkSubject) (*model.TrustMarkSubject, error) {
				return subject, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		body := `{"claim1": "val1"}`
		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(respBody), "claim1") {
			t.Errorf("Expected response to contain 'claim1'")
		}
	})

	t.Run("PutAdditionalClaims_InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(&mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("CopyAdditionalClaims_Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(_ string) (*model.TrustMarkSpec, error) {
				return &model.TrustMarkSpec{AdditionalClaims: map[string]any{"spec_claim": "spec_val"}}, nil
			},
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{AdditionalClaims: map[string]any{"subj_claim": "subj_val"}}, nil
			},
			updateSubjectFn: func(_, _ string, subject *model.TrustMarkSubject) (*model.TrustMarkSubject, error) {
				return subject, nil
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusOK)
		if !strings.Contains(string(respBody), "spec_claim") || !strings.Contains(string(respBody), "subj_claim") {
			t.Errorf("Expected response to contain both claims, got %s", string(respBody))
		}
	})

	t.Run("CopyAdditionalClaims_SpecNotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(_ string) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("spec not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("CopyAdditionalClaims_SubjectNotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(_ string) (*model.TrustMarkSpec, error) {
				return &model.TrustMarkSpec{}, nil
			},
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return nil, model.NotFoundError("subj not found")
			},
		}
		app := setupTrustMarkIssuanceApp(mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNotFound)
	})
}
