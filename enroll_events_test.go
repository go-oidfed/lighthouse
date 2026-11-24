package lighthouse

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/gofiber/fiber/v3"
	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/jwx/v4/jws"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// failingAddEventStore is an event store whose Add always fails.
type failingAddEventStore struct {
	model.SubordinateEventStore
}

func (failingAddEventStore) Add(model.SubordinateEvent) error {
	return errors.New("event store down")
}

// selfSignedEntityServer serves a self-signed entity configuration for the
// server's own URL and returns that URL.
func selfSignedEntityServer(t *testing.T) string {
	t.Helper()
	return selfSignedEntityServerWithMetadata(
		t, &oidfed.Metadata{RelyingParty: &oidfed.OpenIDRelyingPartyMetadata{ClientName: "Enrolling RP"}},
	)
}

// selfSignedEntityServerWithMetadata is selfSignedEntityServer with the
// given metadata in the entity configuration.
func selfSignedEntityServerWithMetadata(t *testing.T, metadata *oidfed.Metadata) string {
	t.Helper()
	sk := rsaKey(t)
	var entityID string
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != oidfedconst.FederationSuffix {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", oidfedconst.ContentTypeEntityStatement)
				_, _ = w.Write(signEnrollingEntityConfiguration(t, sk, entityID, metadata))
			},
		),
	)
	t.Cleanup(srv.Close)
	entityID = srv.URL
	return entityID
}

// signEnrollingEntityConfiguration signs a self-issued entity configuration
// for entityID with sk and the given metadata.
func signEnrollingEntityConfiguration(
	t *testing.T, sk jwx.SigningKey, entityID string, metadata *oidfed.Metadata,
) []byte {
	t.Helper()
	signer := jwx.NewSingleKeyVersatileSigner(sk, jwa.RS256())
	gs := jwx.NewGeneralJWTSigner(signer, []jwa.SignatureAlgorithm{jwa.RS256()})
	pub, err := jwk.PublicKeyOf(sk)
	require.NoError(t, err)
	require.NoError(t, jwk.AssignKeyID(pub))
	kid, _ := pub.KeyID()
	headers := jws.NewHeaders()
	require.NoError(t, headers.Set(jws.KeyIDKey, kid))
	now := time.Now()
	jwt, err := gs.JWTWithHeaders(
		oidfed.EntityStatementPayload{
			Issuer:    entityID,
			Subject:   entityID,
			IssuedAt:  unixtime.Unixtime{Time: now},
			ExpiresAt: unixtime.Unixtime{Time: now.Add(24 * time.Hour)},
			JWKS:      pubJWKS(t, sk),
			Metadata:  metadata,
		},
		headers,
		oidfedconst.JWTTypeEntityStatement,
	)
	require.NoError(t, err)
	return jwt
}

// setupEnrollEventsApp registers the enroll and enroll request endpoints over
// fresh storage. wrap, when set, wraps the event store the endpoints use; the
// real store is returned for reading events back.
func setupEnrollEventsApp(
	t *testing.T, wrap func(model.SubordinateEventStore) model.SubordinateEventStore,
) (*fiber.App, model.SubordinateStorageBackend, model.SubordinateEventStore) {
	t.Helper()
	st := newTestStorage(t)
	store := st.SubordinateStorage()
	real := st.SubordinateEventsStorage()
	var events model.SubordinateEventStore = real
	if wrap != nil {
		events = wrap(real)
	}
	fed := &LightHouse{
		FederationEntity: stubSigningFedEntity{jwt: []byte("signed-statement-jwt")},
		storages: model.Backends{
			Subordinates: store, SubordinateEvents: events, KV: st.KeyValue(),
		},
		fedMetadata: oidfed.FederationEntityMetadata{},
	}
	require.NoError(t, fed.AddEnrollRequestEndpoint(EndpointConf{Path: "/enroll-request"}, store))
	require.NoError(t, fed.AddEnrollEndpoint(EndpointConf{Path: "/enroll"}, store, nil))
	app := fiber.New()
	app.All("/*", fed.dispatch)
	return app, store, real
}

func enrollURL(path, sub string) string {
	return path + "?sub=" + url.QueryEscape(sub) +
		"&entity_type=openid_relying_party&entity_type=openid_provider"
}

func entityEvents(t *testing.T, events model.SubordinateEventStore, entityID string) []model.SubordinateEvent {
	t.Helper()
	got, total, err := events.Query(model.EventQueryOpts{EntityID: &entityID})
	require.NoError(t, err)
	require.EqualValues(t, len(got), total)
	return got
}

func TestEnrollRequest_RecordsJoinRequested(t *testing.T) {
	entityID := selfSignedEntityServer(t)
	app, store, events := setupEnrollEventsApp(t, nil)

	resp, body := doRequestRaw(t, app, httptest.NewRequest(fiber.MethodGet, enrollURL("/enroll-request", entityID), nil))
	require.Equal(t, fiber.StatusAccepted, resp.StatusCode, "body: %s", body)

	sub, err := store.Get(entityID)
	require.NoError(t, err)
	require.NotNil(t, sub)
	got := entityEvents(t, events, entityID)
	require.Len(t, got, 1)
	ev := got[0]
	assert.Equal(t, model.EventTypeJoinRequested, ev.Type)
	assert.Equal(t, entityID, ev.EntityID)
	assert.Equal(t, sub.ID, ev.SubordinateID)
	assert.Nil(t, ev.Actor, "no actor: the entity asked")
	require.NotNil(t, ev.Status)
	assert.Equal(t, model.StatusPending.String(), *ev.Status)
	require.NotNil(t, ev.Message)
	assert.Equal(t, "requested to join as openid_relying_party, openid_provider", *ev.Message)

	// A repeated request while pending records nothing.
	resp, body = doRequestRaw(t, app, httptest.NewRequest(fiber.MethodGet, enrollURL("/enroll-request", entityID), nil))
	require.Equal(t, fiber.StatusAccepted, resp.StatusCode, "body: %s", body)
	assert.Len(t, entityEvents(t, events, entityID), 1)
}

func TestEnroll_RecordsCreated(t *testing.T) {
	entityID := selfSignedEntityServer(t)
	app, store, events := setupEnrollEventsApp(t, nil)

	resp, body := doRequestRaw(t, app, httptest.NewRequest(fiber.MethodGet, enrollURL("/enroll", entityID), nil))
	require.Equal(t, fiber.StatusCreated, resp.StatusCode, "body: %s", body)

	sub, err := store.Get(entityID)
	require.NoError(t, err)
	require.NotNil(t, sub)
	got := entityEvents(t, events, entityID)
	require.Len(t, got, 1)
	ev := got[0]
	assert.Equal(t, model.EventTypeCreated, ev.Type)
	assert.Equal(t, entityID, ev.EntityID)
	assert.Equal(t, sub.ID, ev.SubordinateID)
	assert.Nil(t, ev.Actor, "no actor: the entity enrolled itself")
	require.NotNil(t, ev.Status)
	assert.Equal(t, model.StatusActive.String(), *ev.Status)
	require.NotNil(t, ev.Message)
	assert.Equal(t, "enrolled as openid_relying_party, openid_provider", *ev.Message)

	// Enrolling again as an active subordinate records nothing.
	resp, body = doRequestRaw(t, app, httptest.NewRequest(fiber.MethodGet, enrollURL("/enroll", entityID), nil))
	require.Equal(t, fiber.StatusCreated, resp.StatusCode, "body: %s", body)
	assert.Len(t, entityEvents(t, events, entityID), 1)
}

// TestEnroll_FailingEventStoreKeepsResult: the public endpoints record events
// best-effort, so a failing event store changes neither the answer nor the
// stored subordinate.
func TestEnroll_FailingEventStoreKeepsResult(t *testing.T) {
	for _, tt := range []struct {
		path   string
		status int
		stored model.Status
	}{
		{"/enroll-request", fiber.StatusAccepted, model.StatusPending},
		{"/enroll", fiber.StatusCreated, model.StatusActive},
	} {
		t.Run(
			tt.path, func(t *testing.T) {
				entityID := selfSignedEntityServer(t)
				app, store, events := setupEnrollEventsApp(
					t, func(real model.SubordinateEventStore) model.SubordinateEventStore {
						return failingAddEventStore{real}
					},
				)

				resp, body := doRequestRaw(t, app, httptest.NewRequest(fiber.MethodGet, enrollURL(tt.path, entityID), nil))
				require.Equal(t, tt.status, resp.StatusCode, "body: %s", body)
				sub, err := store.Get(entityID)
				require.NoError(t, err)
				require.NotNil(t, sub)
				assert.Equal(t, tt.stored, sub.Status)
				assert.Empty(t, entityEvents(t, events, entityID))
			},
		)
	}
}

// TestEnroll_MessageWithoutEntityTypes: when neither the request nor the
// entity configuration names an entity type, the message leaves out the
// "as ..." part instead of ending in "as ".
func TestEnroll_MessageWithoutEntityTypes(t *testing.T) {
	for _, tt := range []struct {
		path    string
		status  int
		message string
	}{
		{"/enroll-request", fiber.StatusAccepted, "requested to join"},
		{"/enroll", fiber.StatusCreated, "enrolled"},
	} {
		t.Run(
			tt.path, func(t *testing.T) {
				entityID := selfSignedEntityServerWithMetadata(t, &oidfed.Metadata{})
				app, _, events := setupEnrollEventsApp(t, nil)

				resp, body := doRequestRaw(
					t, app, httptest.NewRequest(fiber.MethodGet, tt.path+"?sub="+url.QueryEscape(entityID), nil),
				)
				require.Equal(t, tt.status, resp.StatusCode, "body: %s", body)
				got := entityEvents(t, events, entityID)
				require.Len(t, got, 1)
				require.NotNil(t, got[0].Message)
				assert.Equal(t, tt.message, *got[0].Message)
			},
		)
	}
}
