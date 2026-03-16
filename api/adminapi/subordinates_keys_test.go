package adminapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-oidfed/lib/jwx"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v3/jwk"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// setupSubordinateKeysApp creates a Fiber app and registers keys endpoints.
func setupSubordinateKeysApp(t *testing.T) (*fiber.App, model.Backends) {
	t.Helper()
	store := newSubordinateTestStorage(t)

	backends := model.Backends{
		Subordinates:      store.SubordinateStorage(),
		SubordinateEvents: store.SubordinateEventsStorage(),
		KV:                store.KeyValue(),
		Transaction: func(fn model.TransactionFunc) error {
			return fn(&model.Backends{
				Subordinates:      store.SubordinateStorage(),
				SubordinateEvents: store.SubordinateEventsStorage(),
				KV:                store.KeyValue(),
			})
		},
	}

	app := fiber.New()
	registerSubordinateKeys(app, backends)
	return app, backends
}

// createTestKey creates a generic RS256 JWK for testing
func createTestKey(kid string) jwk.Key {
	raw := fmt.Sprintf(`{"kty":"RSA","kid":%q,"n":"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw","e":"AQAB"}`, kid)
	k, err := jwk.ParseKey([]byte(raw))
	if err != nil {
		panic(fmt.Sprintf("failed to parse test JWK: %v", err))
	}
	return k
}

// --- GET, PUT, POST /subordinates/:subordinateID/jwks TESTS ---

func TestSubordinateJWKS(t *testing.T) {
	t.Parallel()
	t.Run("GET Success/WithKeys", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateKeysApp(t)

		set := jwk.NewSet()
		key := createTestKey("key-1")
		set.AddKey(key)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://jwks-get.example.org",
			},
			JWKS: model.JWKS{Keys: jwx.JWKS{Set: set}},
		})
		saved, err := backends.Subordinates.Get("https://jwks-get.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/jwks", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		keys, ok := result["keys"].([]any)
		if !ok || len(keys) != 1 {
			t.Errorf("Failed to retrieve JWKS correctly: %+v", result)
		}
	})

	t.Run("GET Success/NoKeys", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateKeysApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://jwks-empty.example.org",
			},
			// No JWKS set
		})
		saved, err := backends.Subordinates.Get("https://jwks-empty.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/jwks", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		keys, ok := result["keys"].([]any)
		if !ok || len(keys) != 0 {
			t.Errorf("Expected empty keys array, got: %+v", result)
		}
	})

	t.Run("GET NotFound", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateKeysApp(t)
		req := httptest.NewRequest("GET", "/subordinates/9999/jwks", http.NoBody)
		resp, _ := doRequest(t, app, req)
		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateKeysApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://jwks-put.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://jwks-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		body := `{"keys":[{"kty":"RSA","kid":"new-key","n":"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw","e":"AQAB"}]}`
		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/jwks", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)

		updated, err := backends.Subordinates.Get("https://jwks-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		if updated.JWKS.Keys.Set == nil || updated.JWKS.Keys.Len() != 1 {
			t.Errorf("Expected JWKS to be replaced in DB")
		}

		// Verify Event
		events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}
		found := false
		for _, e := range events {
			if e.Type == model.EventTypeJWKSReplaced {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected JWKSReplaced event to be logged")
		}
	})

	t.Run("PUT InvalidBody", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateKeysApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://jwks-bad-put.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://jwks-bad-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/jwks", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("POST Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateKeysApp(t)

		set := jwk.NewSet()
		set.AddKey(createTestKey("old-key"))

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://jwks-post.example.org",
			},
			JWKS: model.JWKS{Keys: jwx.JWKS{Set: set}},
		})
		saved, err := backends.Subordinates.Get("https://jwks-post.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		body := `{"kty":"RSA","kid":"new-key","n":"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw","e":"AQAB"}`
		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/jwks", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusCreated)

		updated, err := backends.Subordinates.Get("https://jwks-post.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		if updated.JWKS.Keys.Set == nil || updated.JWKS.Keys.Len() != 2 {
			t.Errorf("Expected new JWK to be merged into DB, got length %d", updated.JWKS.Keys.Len())
		}

		// Verify Event
		events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}
		found := false
		for _, e := range events {
			if e.Type == model.EventTypeJWKAdded {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected JWKAdded event to be logged")
		}
	})

	t.Run("POST InvalidBody", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateKeysApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://jwks-bad-post.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://jwks-bad-post.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/jwks", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})
}

// --- DELETE /subordinates/:subordinateID/jwks/:kid TESTS ---

func TestSubordinateJWKDelete(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateKeysApp(t)

		set := jwk.NewSet()
		set.AddKey(createTestKey("keep-me"))
		set.AddKey(createTestKey("delete-me"))

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://jwk-delete.example.org",
			},
			JWKS: model.JWKS{Keys: jwx.JWKS{Set: set}},
		})
		saved, err := backends.Subordinates.Get("https://jwk-delete.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/jwks/delete-me", saved.ID), http.NoBody)
		resp, _ := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusNoContent)

		updated, err := backends.Subordinates.Get("https://jwk-delete.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		if updated.JWKS.Keys.Set == nil || updated.JWKS.Keys.Len() != 1 {
			t.Fatalf("Expected exactly 1 key remaining, got %d", updated.JWKS.Keys.Len())
		}
		key, ok := updated.JWKS.Keys.Key(0)
		kid, _ := key.KeyID()
		if !ok || kid != "keep-me" {
			t.Errorf("Expected remaining key to be keep-me, got %v", kid)
		}

		// Verify Event
		events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}
		found := false
		for _, e := range events {
			if e.Type == model.EventTypeJWKRemoved {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected JWKRemoved event to be logged")
		}
	})

	t.Run("NotFound/Subordinate", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateKeysApp(t)
		req := httptest.NewRequest("DELETE", "/subordinates/9999/jwks/delete-me", http.NoBody)
		resp, _ := doRequest(t, app, req)
		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})

	t.Run("NotFound/Key", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateKeysApp(t)

		set := jwk.NewSet()
		set.AddKey(createTestKey("keep-me"))

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://jwk-delete-missing.example.org",
			},
			JWKS: model.JWKS{Keys: jwx.JWKS{Set: set}},
		})
		saved, err := backends.Subordinates.Get("https://jwk-delete-missing.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/jwks/missing-kid", saved.ID), http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusNoContent)
	})
}
