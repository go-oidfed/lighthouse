package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// --- MOCKS ---

type mockTrustMarkSpecStore struct {
	model.TrustMarkSpecStore
	listFn   func() ([]model.TrustMarkSpec, error)
	createFn func(spec *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error)
	getFn    func(id string) (*model.TrustMarkSpec, error)
	updateFn func(id string, spec *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error)
	patchFn  func(id string, updates map[string]any) (*model.TrustMarkSpec, error)
	deleteFn func(id string) error

	listSubjectsFn        func(specID string, status *model.Status) ([]model.TrustMarkSubject, error)
	createSubjectFn       func(specID string, subject *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error)
	getSubjectFn          func(specID, subjectID string) (*model.TrustMarkSubject, error)
	updateSubjectFn       func(specID, subjectID string, subject *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error)
	deleteSubjectFn       func(specID, subjectID string) error
	changeSubjectStatusFn func(specID, subjectID string, status model.Status) (*model.TrustMarkSubject, error)
}

func (m *mockTrustMarkSpecStore) List() ([]model.TrustMarkSpec, error) {
	if m.listFn != nil {
		return m.listFn()
	}
	return nil, nil
}
func (m *mockTrustMarkSpecStore) Create(spec *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error) {
	if m.createFn != nil {
		return m.createFn(spec)
	}
	return &model.TrustMarkSpec{
		TrustMarkType: spec.TrustMarkType,
	}, nil
}
func (m *mockTrustMarkSpecStore) Get(id string) (*model.TrustMarkSpec, error) {
	if m.getFn != nil {
		return m.getFn(id)
	}
	return nil, nil
}
func (m *mockTrustMarkSpecStore) Update(id string, spec *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error) {
	if m.updateFn != nil {
		return m.updateFn(id, spec)
	}
	return &model.TrustMarkSpec{
		TrustMarkType: spec.TrustMarkType,
	}, nil
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
func (m *mockTrustMarkSpecStore) CreateSubject(specID string, subject *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
	if m.createSubjectFn != nil {
		return m.createSubjectFn(specID, subject)
	}
	return &model.TrustMarkSubject{
		EntityID: subject.EntityID,
	}, nil
}
func (m *mockTrustMarkSpecStore) GetSubject(specID, subjectID string) (*model.TrustMarkSubject, error) {
	if m.getSubjectFn != nil {
		return m.getSubjectFn(specID, subjectID)
	}
	return nil, nil
}
func (m *mockTrustMarkSpecStore) UpdateSubject(specID, subjectID string, subject *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
	if m.updateSubjectFn != nil {
		return m.updateSubjectFn(specID, subjectID, subject)
	}
	return &model.TrustMarkSubject{
		EntityID: subject.EntityID,
	}, nil
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

func setupTrustMarkIssuanceApp(t *testing.T, store model.TrustMarkSpecStore) *fiber.App {
	t.Helper()
	app := fiber.New()
	registerTrustMarkIssuance(app, store, nil)
	return app
}

func setupRealTrustMarkIssuanceApp(t *testing.T) (*fiber.App, model.TrustMarkSpecStore) {
	t.Helper()
	store := newTestStorage(t)
	specStore := store.TrustMarkSpecStorage()
	return setupTrustMarkIssuanceApp(t, specStore), specStore
}

func requireJSONMap(t *testing.T, value any, name string) map[string]any {
	t.Helper()
	m, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected %s to be map[string]any, got %T", name, value)
	}
	return m
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)
		var specs []model.TrustMarkSpec
		if err := json.Unmarshal(body, &specs); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(specs) != 1 || specs[0].TrustMarkType != "type1" {
			t.Errorf("expected 1 spec with TrustMarkType 'type1', got %+v", specs)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			listFn: func() ([]model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
	})
}

func TestTrustMarkSpecHandlers_Create(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			createFn: func(spec *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error) {
				return &model.TrustMarkSpec{TrustMarkType: spec.TrustMarkType}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		body := `{"trust_mark_type": "type1"}`
		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusCreated)
		var spec model.TrustMarkSpec
		if err := json.Unmarshal(respBody, &spec); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if spec.TrustMarkType != "type1" {
			t.Errorf("expected TrustMarkType 'type1', got %q", spec.TrustMarkType)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`invalid json`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("MissingType", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			createFn: func(_ *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, model.AlreadyExistsError("exists")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`{"trust_mark_type": "type1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusConflict, "invalid_request")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			createFn: func(_ *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec", strings.NewReader(`{"trust_mark_type": "type1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)
		var spec model.TrustMarkSpec
		if err := json.Unmarshal(body, &spec); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if spec.TrustMarkType != "type1" {
			t.Errorf("expected TrustMarkType 'type1', got %q", spec.TrustMarkType)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(_ string) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(_ string) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
	})
}

func TestTrustMarkSpecHandlers_Update(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateFn: func(_ string, spec *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error) {
				return &model.TrustMarkSpec{TrustMarkType: spec.TrustMarkType}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		body := `{"trust_mark_type": "type2"}`
		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusOK)
		var spec model.TrustMarkSpec
		if err := json.Unmarshal(respBody, &spec); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if spec.TrustMarkType != "type2" {
			t.Errorf("expected TrustMarkType 'type2', got %q", spec.TrustMarkType)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("MissingType", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateFn: func(_ string, _ *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateFn: func(_ string, _ *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		body := `{"trust_mark_type": "type3"}`
		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusOK)
		var spec model.TrustMarkSpec
		if err := json.Unmarshal(respBody, &spec); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if spec.TrustMarkType != "type3" {
			t.Errorf("expected TrustMarkType 'type3', got %q", spec.TrustMarkType)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			patchFn: func(_ string, _ map[string]any) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type3"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			patchFn: func(_ string, _ map[string]any) (*model.TrustMarkSpec, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("PATCH", "/trust-marks/issuance-spec/1", strings.NewReader(`{"trust_mark_type": "type3"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)

		requireStatus(t, resp, bodyBytes, http.StatusNoContent)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			deleteFn: func(_ string) error {
				return model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			deleteFn: func(_ string) error {
				return errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)
		var subjects []model.TrustMarkSubject
		if err := json.Unmarshal(body, &subjects); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(subjects) != 1 || subjects[0].EntityID != "sub1" {
			t.Errorf("expected 1 subject with EntityID 'sub1', got %+v", subjects)
		}
	})

	t.Run("InvalidStatusFilter", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects?status=invalid", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			listSubjectsFn: func(_ string, _ *model.Status) ([]model.TrustMarkSubject, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
	})
}

func TestTrustMarkSubjectHandlers_Create(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			createSubjectFn: func(_ string, subject *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{EntityID: subject.EntityID}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		body := `{"entity_id": "sub1"}`
		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusCreated)
		var subject model.TrustMarkSubject
		if err := json.Unmarshal(respBody, &subject); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if subject.EntityID != "sub1" {
			t.Errorf("expected EntityID 'sub1', got %q", subject.EntityID)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("MissingEntityID", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			createSubjectFn: func(_ string, _ *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects", strings.NewReader(`{"entity_id": "sub1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		t.Parallel()
		app, specStore := setupRealTrustMarkIssuanceApp(t)

		specType := "type-already-exists"
		subjectID := "subject-already-exists"

		_, err := specStore.Create(&model.AddTrustMarkSpec{
			TrustMarkType: specType,
		})
		if err != nil {
			t.Fatalf("failed to seed spec: %v", err)
		}

		_, err = specStore.CreateSubject(specType, &model.AddTrustMarkSubject{
			EntityID: subjectID,
			Status:   model.StatusActive,
		})
		if err != nil {
			t.Fatalf("failed to seed subject: %v", err)
		}

		body := `{"entity_id": "` + subjectID + `", "status": "pending"}`
		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/"+specType+"/subjects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		assertErrorResponse(t, resp, respBody, http.StatusConflict, "invalid_request")
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)
		var subject model.TrustMarkSubject
		if err := json.Unmarshal(body, &subject); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if subject.EntityID != "sub1" {
			t.Errorf("expected EntityID 'sub1', got %q", subject.EntityID)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
	})
}

func TestTrustMarkSubjectHandlers_Update(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateSubjectFn: func(_, _ string, subject *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{EntityID: subject.EntityID}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		body := `{"entity_id": "sub2"}`
		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusOK)
		var subject model.TrustMarkSubject
		if err := json.Unmarshal(respBody, &subject); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if subject.EntityID != "sub2" {
			t.Errorf("expected EntityID 'sub2', got %q", subject.EntityID)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("MissingEntityID", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateSubjectFn: func(_, _ string, _ *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2", strings.NewReader(`{"entity_id": "sub2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			updateSubjectFn: func(_, _ string, _ *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2", strings.NewReader(`{"entity_id": "sub2"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)

		requireStatus(t, resp, bodyBytes, http.StatusNoContent)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			deleteSubjectFn: func(_, _ string) error {
				return model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			deleteSubjectFn: func(_, _ string) error {
				return errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/1/subjects/2", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/status", strings.NewReader("inactive"))
		req.Header.Set("Content-Type", "text/plain")
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)
		var subject model.TrustMarkSubject
		if err := json.Unmarshal(body, &subject); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if subject.Status != model.StatusInactive {
			t.Errorf("expected Status 'inactive', got %q", subject.Status)
		}
	})

	t.Run("MissingStatus", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/status", strings.NewReader("   "))
		req.Header.Set("Content-Type", "text/plain")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("InvalidStatus", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/status", strings.NewReader("unknown"))
		req.Header.Set("Content-Type", "text/plain")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			changeSubjectStatusFn: func(_, _ string, _ model.Status) (*model.TrustMarkSubject, error) {
				return nil, model.NotFoundError("not found")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/status", strings.NewReader("active"))
		req.Header.Set("Content-Type", "text/plain")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			changeSubjectStatusFn: func(_, _ string, _ model.Status) (*model.TrustMarkSubject, error) {
				return nil, errors.New("db error")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/status", strings.NewReader("active"))
		req.Header.Set("Content-Type", "text/plain")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)
		var claims map[string]any
		if err := json.Unmarshal(body, &claims); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if claims["claim1"] != "val1" {
			t.Errorf("expected claim1='val1', got %+v", claims)
		}
	})

	t.Run("GetAdditionalClaims_SuccessEmptyClaims", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{AdditionalClaims: nil}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("GET", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	})

	t.Run("PutAdditionalClaims_Success", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getSubjectFn: func(_, _ string) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{}, nil
			},
			updateSubjectFn: func(_, _ string, subject *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{EntityID: subject.EntityID, AdditionalClaims: subject.AdditionalClaims}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		body := `{"claim1": "val1"}`
		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusOK)
		var claims map[string]any
		if err := json.Unmarshal(respBody, &claims); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if claims["claim1"] != "val1" {
			t.Errorf("expected claim1='val1', got %+v", claims)
		}
	})

	t.Run("PutAdditionalClaims_InvalidBody", func(t *testing.T) {
		t.Parallel()
		app := setupTrustMarkIssuanceApp(t, &mockTrustMarkSpecStore{})

		req := httptest.NewRequest("PUT", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", strings.NewReader(`invalid`))
		req.Header.Set("Content-Type", "application/json")
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
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
			updateSubjectFn: func(_, _ string, subject *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
				return &model.TrustMarkSubject{EntityID: subject.EntityID, AdditionalClaims: subject.AdditionalClaims}, nil
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusOK)
		var claims map[string]any
		if err := json.Unmarshal(respBody, &claims); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if claims["spec_claim"] != "spec_val" || claims["subj_claim"] != "subj_val" {
			t.Errorf("expected both claims, got %+v", claims)
		}
	})

	t.Run("CopyAdditionalClaims_SpecNotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := &mockTrustMarkSpecStore{
			getFn: func(_ string) (*model.TrustMarkSpec, error) {
				return nil, model.NotFoundError("spec not found")
			},
		}
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
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
		app := setupTrustMarkIssuanceApp(t, mockStore)

		req := httptest.NewRequest("POST", "/trust-marks/issuance-spec/1/subjects/2/additional-claims", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	})
}

func TestTrustMarkSpecHandlers_RealStoragePersistence(t *testing.T) {
	t.Parallel()

	t.Run("CreatePersistsStructuredFields", func(t *testing.T) {
		t.Parallel()
		app, specStore := setupRealTrustMarkIssuanceApp(t)

		body := `{
			"trust_mark_type": "type-real-create",
			"additional_claims": {
				"profile": {
					"name": "gold",
					"enabled": true
				}
			},
			"eligibility_config": {
				"mode": "custom",
				"checker": {
					"type": "http",
					"config": {
						"endpoint": "https://checker.example.org"
					}
				},
				"check_cache_ttl": 90
			}
		}`

		req := httptest.NewRequest(http.MethodPost, "/trust-marks/issuance-spec", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusCreated)

		var created model.TrustMarkSpec
		if err := json.Unmarshal(respBody, &created); err != nil {
			t.Fatalf("failed to unmarshal create response: %v", err)
		}
		if created.TrustMarkType != "type-real-create" {
			t.Fatalf("expected trust_mark_type %q, got %q", "type-real-create", created.TrustMarkType)
		}
		responseProfile := requireJSONMap(t, created.AdditionalClaims["profile"], "create response additional_claims.profile")
		if responseProfile["name"] != "gold" || responseProfile["enabled"] != true {
			t.Fatalf("unexpected create response additional_claims.profile: %+v", responseProfile)
		}
		if created.EligibilityConfig == nil || created.EligibilityConfig.Mode != model.EligibilityModeCustom {
			t.Fatalf("unexpected create response eligibility_config: %+v", created.EligibilityConfig)
		}
		if created.EligibilityConfig.Checker == nil || created.EligibilityConfig.Checker.Type != "http" {
			t.Fatalf("unexpected create response checker config: %+v", created.EligibilityConfig)
		}

		persisted, err := specStore.Get("type-real-create")
		if err != nil {
			t.Fatalf("failed to reload created spec: %v", err)
		}
		persistedProfile := requireJSONMap(t, persisted.AdditionalClaims["profile"], "persisted additional_claims.profile")
		if persistedProfile["name"] != "gold" || persistedProfile["enabled"] != true {
			t.Fatalf("unexpected persisted additional_claims.profile: %+v", persistedProfile)
		}
		if persisted.EligibilityConfig == nil || persisted.EligibilityConfig.Mode != model.EligibilityModeCustom {
			t.Fatalf("unexpected persisted eligibility_config: %+v", persisted.EligibilityConfig)
		}
		if persisted.EligibilityConfig.Checker == nil || persisted.EligibilityConfig.Checker.Config["endpoint"] != "https://checker.example.org" {
			t.Fatalf("unexpected persisted checker config: %+v", persisted.EligibilityConfig)
		}
	})

	t.Run("UpdatePersistsStructuredFields", func(t *testing.T) {
		t.Parallel()
		app, specStore := setupRealTrustMarkIssuanceApp(t)

		if _, err := specStore.Create(&model.AddTrustMarkSpec{TrustMarkType: "type-real-update-initial"}); err != nil {
			t.Fatalf("failed to seed spec: %v", err)
		}

		body := `{
			"trust_mark_type": "type-real-update-final",
			"description": "updated spec",
			"additional_claims": {
				"profile": {
					"name": "platinum"
				}
			},
			"eligibility_config": {
				"mode": "custom",
				"checker": {
					"type": "script",
					"config": {
						"path": "/opt/checker.sh"
					}
				},
				"check_cache_ttl": 45
			}
		}`

		req := httptest.NewRequest(http.MethodPut, "/trust-marks/issuance-spec/type-real-update-initial", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusOK)

		var updated model.TrustMarkSpec
		if err := json.Unmarshal(respBody, &updated); err != nil {
			t.Fatalf("failed to unmarshal update response: %v", err)
		}
		if updated.TrustMarkType != "type-real-update-final" || updated.Description != "updated spec" {
			t.Fatalf("unexpected update response payload: %+v", updated)
		}
		updatedProfile := requireJSONMap(t, updated.AdditionalClaims["profile"], "update response additional_claims.profile")
		if updatedProfile["name"] != "platinum" {
			t.Fatalf("unexpected update response additional_claims.profile: %+v", updatedProfile)
		}

		persisted, err := specStore.Get("type-real-update-final")
		if err != nil {
			t.Fatalf("failed to reload updated spec: %v", err)
		}
		persistedProfile := requireJSONMap(t, persisted.AdditionalClaims["profile"], "persisted update additional_claims.profile")
		if persisted.Description != "updated spec" || persistedProfile["name"] != "platinum" {
			t.Fatalf("unexpected persisted updated spec: %+v", persisted)
		}
		if persisted.EligibilityConfig == nil || persisted.EligibilityConfig.Checker == nil || persisted.EligibilityConfig.Checker.Config["path"] != "/opt/checker.sh" {
			t.Fatalf("unexpected persisted updated eligibility_config: %+v", persisted.EligibilityConfig)
		}
	})

	t.Run("PatchPersistsStructuredFields", func(t *testing.T) {
		t.Parallel()
		app, specStore := setupRealTrustMarkIssuanceApp(t)

		if _, err := specStore.Create(&model.AddTrustMarkSpec{TrustMarkType: "type-real-patch"}); err != nil {
			t.Fatalf("failed to seed patch spec: %v", err)
		}

		body := `{
			"additional_claims": {
				"profile": {
					"name": "silver",
					"tier": "b"
				}
			},
			"eligibility_config": {
				"mode": "custom",
				"checker": {
					"type": "http",
					"config": {
						"endpoint": "https://patched-checker.example.org"
					}
				},
				"check_cache_ttl": 30
			}
		}`

		req := httptest.NewRequest(http.MethodPatch, "/trust-marks/issuance-spec/type-real-patch", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusOK)

		var patched model.TrustMarkSpec
		if err := json.Unmarshal(respBody, &patched); err != nil {
			t.Fatalf("failed to unmarshal patch response: %v", err)
		}
		patchedProfile := requireJSONMap(t, patched.AdditionalClaims["profile"], "patch response additional_claims.profile")
		if patched.TrustMarkType != "type-real-patch" || patchedProfile["name"] != "silver" || patchedProfile["tier"] != "b" {
			t.Fatalf("unexpected patch response payload: %+v", patched)
		}

		persisted, err := specStore.Get("type-real-patch")
		if err != nil {
			t.Fatalf("failed to reload patched spec: %v", err)
		}
		persistedProfile := requireJSONMap(t, persisted.AdditionalClaims["profile"], "persisted patch additional_claims.profile")
		if persistedProfile["name"] != "silver" || persistedProfile["tier"] != "b" {
			t.Fatalf("unexpected persisted patch additional_claims.profile: %+v", persistedProfile)
		}
		if persisted.EligibilityConfig == nil || persisted.EligibilityConfig.Checker == nil || persisted.EligibilityConfig.Checker.Config["endpoint"] != "https://patched-checker.example.org" {
			t.Fatalf("unexpected persisted patch eligibility_config: %+v", persisted.EligibilityConfig)
		}
	})
}

func TestTrustMarkSubjectHandlers_RealStoragePersistence(t *testing.T) {
	t.Parallel()

	t.Run("CreatePersistsAdditionalClaims", func(t *testing.T) {
		t.Parallel()
		app, specStore := setupRealTrustMarkIssuanceApp(t)

		if _, err := specStore.Create(&model.AddTrustMarkSpec{TrustMarkType: "type-subject-create"}); err != nil {
			t.Fatalf("failed to seed subject spec: %v", err)
		}

		body := `{
			"entity_id": "subject-create",
			"additional_claims": {
				"profile": {
					"sector": "finance"
				},
				"enabled": true
			}
		}`

		req := httptest.NewRequest(http.MethodPost, "/trust-marks/issuance-spec/type-subject-create/subjects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusCreated)

		var created model.TrustMarkSubject
		if err := json.Unmarshal(respBody, &created); err != nil {
			t.Fatalf("failed to unmarshal subject create response: %v", err)
		}
		if created.EntityID != "subject-create" {
			t.Fatalf("unexpected subject create response: %+v", created)
		}
		createdProfile := requireJSONMap(t, created.AdditionalClaims["profile"], "subject create response additional_claims.profile")
		if createdProfile["sector"] != "finance" || created.AdditionalClaims["enabled"] != true {
			t.Fatalf("unexpected subject create additional claims: %+v", created.AdditionalClaims)
		}

		persisted, err := specStore.GetSubject("type-subject-create", "subject-create")
		if err != nil {
			t.Fatalf("failed to reload created subject: %v", err)
		}
		persistedProfile := requireJSONMap(t, persisted.AdditionalClaims["profile"], "persisted subject create additional_claims.profile")
		if persistedProfile["sector"] != "finance" || persisted.AdditionalClaims["enabled"] != true {
			t.Fatalf("unexpected persisted subject additional claims: %+v", persisted.AdditionalClaims)
		}
	})

	t.Run("UpdatePersistsAdditionalClaims", func(t *testing.T) {
		t.Parallel()
		app, specStore := setupRealTrustMarkIssuanceApp(t)

		if _, err := specStore.Create(&model.AddTrustMarkSpec{TrustMarkType: "type-subject-update"}); err != nil {
			t.Fatalf("failed to seed subject update spec: %v", err)
		}
		if _, err := specStore.CreateSubject("type-subject-update", &model.AddTrustMarkSubject{EntityID: "subject-update", Status: model.StatusActive}); err != nil {
			t.Fatalf("failed to seed subject: %v", err)
		}

		body := `{
			"entity_id": "subject-update",
			"description": "updated subject",
			"additional_claims": {
				"profile": {
					"sector": "education"
				},
				"flags": {
					"beta": true
				}
			}
		}`

		req := httptest.NewRequest(http.MethodPut, "/trust-marks/issuance-spec/type-subject-update/subjects/subject-update", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusOK)

		var updated model.TrustMarkSubject
		if err := json.Unmarshal(respBody, &updated); err != nil {
			t.Fatalf("failed to unmarshal subject update response: %v", err)
		}
		updatedFlags := requireJSONMap(t, updated.AdditionalClaims["flags"], "subject update response additional_claims.flags")
		if updated.Description != "updated subject" || updatedFlags["beta"] != true {
			t.Fatalf("unexpected subject update response: %+v", updated)
		}

		persisted, err := specStore.GetSubject("type-subject-update", "subject-update")
		if err != nil {
			t.Fatalf("failed to reload updated subject: %v", err)
		}
		persistedProfile := requireJSONMap(t, persisted.AdditionalClaims["profile"], "persisted subject update additional_claims.profile")
		if persisted.Description != "updated subject" || persistedProfile["sector"] != "education" {
			t.Fatalf("unexpected persisted updated subject: %+v", persisted)
		}
	})

	t.Run("PutAdditionalClaimsPersistsStructuredPayload", func(t *testing.T) {
		t.Parallel()
		app, specStore := setupRealTrustMarkIssuanceApp(t)

		if _, err := specStore.Create(&model.AddTrustMarkSpec{TrustMarkType: "type-subject-put-claims"}); err != nil {
			t.Fatalf("failed to seed put-additional-claims spec: %v", err)
		}
		if _, err := specStore.CreateSubject("type-subject-put-claims", &model.AddTrustMarkSubject{EntityID: "subject-put-claims", Status: model.StatusActive}); err != nil {
			t.Fatalf("failed to seed put-additional-claims subject: %v", err)
		}

		body := `{
			"profile": {
				"sector": "health"
			},
			"enabled": true
		}`

		req := httptest.NewRequest(http.MethodPut, "/trust-marks/issuance-spec/type-subject-put-claims/subjects/subject-put-claims/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusOK)

		var updatedClaims map[string]any
		if err := json.Unmarshal(respBody, &updatedClaims); err != nil {
			t.Fatalf("failed to unmarshal put-additional-claims response: %v", err)
		}
		responseProfile := requireJSONMap(t, updatedClaims["profile"], "put-additional-claims response profile")
		if responseProfile["sector"] != "health" || updatedClaims["enabled"] != true {
			t.Fatalf("unexpected put-additional-claims response: %+v", updatedClaims)
		}

		persisted, err := specStore.GetSubject("type-subject-put-claims", "subject-put-claims")
		if err != nil {
			t.Fatalf("failed to reload subject after put-additional-claims: %v", err)
		}
		persistedProfile := requireJSONMap(t, persisted.AdditionalClaims["profile"], "persisted put-additional-claims profile")
		if persistedProfile["sector"] != "health" || persisted.AdditionalClaims["enabled"] != true {
			t.Fatalf("unexpected persisted put-additional-claims payload: %+v", persisted.AdditionalClaims)
		}
	})

	t.Run("CopyAdditionalClaimsPersistsMergedPayload", func(t *testing.T) {
		t.Parallel()
		app, specStore := setupRealTrustMarkIssuanceApp(t)

		if _, err := specStore.Create(&model.AddTrustMarkSpec{
			TrustMarkType: "type-subject-copy-claims",
			AdditionalClaims: map[string]any{
				"spec_object": map[string]any{"source": "spec"},
				"shared":      map[string]any{"owner": "spec"},
			},
		}); err != nil {
			t.Fatalf("failed to seed copy-additional-claims spec: %v", err)
		}
		if _, err := specStore.CreateSubject("type-subject-copy-claims", &model.AddTrustMarkSubject{
			EntityID: "subject-copy-claims",
			Status:   model.StatusActive,
			AdditionalClaims: map[string]any{
				"shared":       map[string]any{"owner": "subject"},
				"subject_flag": true,
			},
		}); err != nil {
			t.Fatalf("failed to seed copy-additional-claims subject: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/trust-marks/issuance-spec/type-subject-copy-claims/subjects/subject-copy-claims/additional-claims", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		requireStatus(t, resp, respBody, http.StatusOK)

		var mergedClaims map[string]any
		if err := json.Unmarshal(respBody, &mergedClaims); err != nil {
			t.Fatalf("failed to unmarshal copy-additional-claims response: %v", err)
		}
		mergedSpecObject := requireJSONMap(t, mergedClaims["spec_object"], "copy-additional-claims response spec_object")
		mergedShared := requireJSONMap(t, mergedClaims["shared"], "copy-additional-claims response shared")
		if mergedSpecObject["source"] != "spec" || mergedShared["owner"] != "subject" || mergedClaims["subject_flag"] != true {
			t.Fatalf("unexpected merged claims response: %+v", mergedClaims)
		}

		persisted, err := specStore.GetSubject("type-subject-copy-claims", "subject-copy-claims")
		if err != nil {
			t.Fatalf("failed to reload subject after copy-additional-claims: %v", err)
		}
		persistedShared := requireJSONMap(t, persisted.AdditionalClaims["shared"], "persisted copy-additional-claims shared")
		if persistedShared["owner"] != "subject" || persisted.AdditionalClaims["subject_flag"] != true {
			t.Fatalf("unexpected persisted merged claims: %+v", persisted.AdditionalClaims)
		}
	})

	t.Run("MemoryIsolation", func(t *testing.T) {
		t.Parallel()
		app, specStore := setupRealTrustMarkIssuanceApp(t)

		// 1. Create a Trust Mark Spec with nested map additional claims
		specType := "type-memory-isolation"
		subjectID := "subject-memory-isolation"

		_, err := specStore.Create(&model.AddTrustMarkSpec{
			TrustMarkType: specType,
			AdditionalClaims: map[string]any{
				"profile": map[string]any{
					"tier": "gold",
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to seed spec: %v", err)
		}

		_, err = specStore.CreateSubject(specType, &model.AddTrustMarkSubject{
			EntityID: subjectID,
			Status:   model.StatusActive,
		})
		if err != nil {
			t.Fatalf("failed to seed subject: %v", err)
		}

		// 2. Hit the endpoint that copies/merges these claims into a subject
		req := httptest.NewRequest(http.MethodPost, "/trust-marks/issuance-spec/"+specType+"/subjects/"+subjectID+"/additional-claims", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusOK)

		// 3. Mutate the resulting merged claims using a PUT request to the subject
		body := `{
		"profile": {
			"tier": "silver"
		}
	}`
		req = httptest.NewRequest(http.MethodPut, "/trust-marks/issuance-spec/"+specType+"/subjects/"+subjectID+"/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes = doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusOK)

		// 4. CRITICAL ASSERTION: Re-read the original Spec and assert that AdditionalClaims["profile"]["tier"] is STILL "gold"
		spec, err := specStore.Get(specType)
		if err != nil {
			t.Fatalf("failed to retrieve spec: %v", err)
		}

		profile, ok := spec.AdditionalClaims["profile"].(map[string]any)
		if !ok {
			t.Fatalf("expected profile to be map[string]any, got %T", spec.AdditionalClaims["profile"])
		}

		// NOTE: With real storage, the DB provides isolation — Get() returns a
		// fresh deserialized copy each time. The shallow copy in copyAdditionalClaims
		// is a latent risk that would manifest if handlers cached spec objects
		// in memory across requests, but the DB boundary prevents mutation here.
		if profile["tier"] != "gold" {
			t.Errorf("Memory isolation failed: spec tier should remain 'gold', got %v", profile["tier"])
		}
	})
}
