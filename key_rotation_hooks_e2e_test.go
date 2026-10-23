package lighthouse

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/jwx/v4/jws"
	"github.com/stretchr/testify/require"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/go-oidfed/lib/unixtime"

	"github.com/go-oidfed/lighthouse/api/adminapi"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

const testLHUnderTestEntityID = "https://lh-under-test.example"

// staticHintsStore is a minimal AuthorityHintsStore for tests.
type staticHintsStore struct {
	hints []model.AuthorityHint
}

func (s *staticHintsStore) List() ([]model.AuthorityHint, error) { return s.hints, nil }
func (s *staticHintsStore) Create(model.AddAuthorityHint) (*model.AuthorityHint, error) {
	return nil, nil
}
func (s *staticHintsStore) Get(string) (*model.AuthorityHint, error) { return nil, nil }
func (s *staticHintsStore) Update(string, model.AddAuthorityHint) (*model.AuthorityHint, error) {
	return nil, nil
}
func (s *staticHintsStore) Delete(string) error { return nil }

// signECWithMetadata builds a signed Entity Configuration for entityID, signed
// with sk, embedding extra claims in the federation_entity metadata.
func signECWithMetadata(
	t *testing.T, sk jwx.SigningKey, entityID string, extra map[string]any,
) []byte {
	t.Helper()
	pubJWKS := pubJWKS(t, sk)
	signer := jwx.NewSingleKeyVersatileSigner(sk, jwa.RS256())
	gs := jwx.NewGeneralJWTSigner(signer, []jwa.SignatureAlgorithm{jwa.RS256()})
	headers := jws.NewHeaders()
	pub, err := jwk.PublicKeyOf(sk)
	require.NoError(t, err)
	require.NoError(t, jwk.AssignKeyID(pub))
	kid, _ := pub.KeyID()
	_ = headers.Set(jws.KeyIDKey, kid)
	now := time.Now()
	jwtBytes, err := gs.JWTWithHeaders(
		oidfed.EntityStatementPayload{
			Issuer:    entityID,
			Subject:   entityID,
			IssuedAt:  unixtime.Unixtime{Time: now},
			ExpiresAt: unixtime.Unixtime{Time: now.Add(time.Hour)},
			JWKS:      pubJWKS,
			Metadata: &oidfed.Metadata{
				FederationEntity: &oidfed.FederationEntityMetadata{Extra: extra},
			},
		},
		headers,
		oidfedconst.JWTTypeEntityStatement,
	)
	require.NoError(t, err)
	return jwtBytes
}

// newTestKV returns a real SQLite-backed KeyValueStore.
func newTestKV(t *testing.T) model.KeyValueStore {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	st, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	return st.KeyValue()
}

// newTestKMS builds a real rotation-capable KMS with keys loaded and rotation
// NOT started (mirrors initKey after the Load step).
func newTestKMS(t *testing.T) (adminapi.KeyManagement, kms.KeyManagementSystem) {
	t.Helper()
	pks := newDBPKStorage(t, "federation")
	kmsys := kms.NewPEMStorageKMS(
		kms.KMSConfig{
			GenerateKeys: true,
			Algs:         []jwa.SignatureAlgorithm{jwa.RS256()},
			RSAKeyLen:    2048,
			KeyRotation:  kms.KeyRotationConfig{Enabled: false},
			EntityID:     testLHUnderTestEntityID,
		},
		&kms.FilesystemPEMStorage{Dir: t.TempDir()},
		&kms.FilesystemStateStorer{Dir: t.TempDir()},
		pks,
	)
	require.NoError(t, kmsys.Load())
	return adminapi.KeyManagement{KMS: "db", Keys: kmsys, BasicKeys: kmsys, KMSManagedPKs: pks, APIManagedPKs: pks}, kmsys
}

// TestJWKSSyncTriggerHookFiresToSuperior verifies the full wiring: a trigger
// authority-hint hook attached via applyAuthorityHintJWKSSyncHooks fires a POST
// to the superior's federation_jwks_update_trigger_endpoint when the KMS
// rotates keys.
func TestJWKSSyncTriggerHookFiresToSuperior(t *testing.T) {
	var triggerHits atomic.Int32
	triggerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		triggerHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer triggerSrv.Close()

	superiorSK := rsaKey(t)
	var ecSrv *httptest.Server
	ecSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ec := signECWithMetadata(t, superiorSK, ecSrv.URL, map[string]any{
			oidfedconst.FederationJWKSUpdateTriggerEndpoint: triggerSrv.URL,
		})
		w.Header().Set("Content-Type", oidfedconst.ContentTypeEntityStatement)
		_, _ = w.Write(ec)
	}))
	defer ecSrv.Close()

	kv := newTestKV(t)
	hints := &staticHintsStore{hints: []model.AuthorityHint{
		{
			EntityID: ecSrv.URL, // superior entity — its EC advertises the trigger endpoint
			JWKSSync: &model.JWKSSyncConf{
				Mode:    model.JWKSSyncTrigger,
				Timeout: 5,
			},
		},
	}}
	keyManagement, _ := newTestKMS(t)

	// Attach hooks (what NewLightHouse does at boot and after admin mutations).
	require.NoError(t, applyAuthorityHintJWKSSyncHooks(testLHUnderTestEntityID, nil, kv, hints, keyManagement))

	// Force a rotation; the KMS fires the attached hooks.
	require.NoError(t, keyManagement.Keys.RotateAllKeys(true, "test"))

	require.Eventually(t, func() bool {
		return triggerHits.Load() > 0
	}, 5*time.Second, 10*time.Millisecond, "expected the superior trigger endpoint to be hit after key rotation")

	// Runtime change: disabling sync (the same `applyAuthorityHintJWKSSyncHooks`
	// the admin API invokes after each authority-hint mutation) must detach the
	// hook so a subsequent rotation does NOT notify the superior.
	hints.hints[0].JWKSSync = nil
	require.NoError(t, applyAuthorityHintJWKSSyncHooks(testLHUnderTestEntityID, nil, kv, hints, keyManagement))
	triggerHits.Store(0)
	require.NoError(t, keyManagement.Keys.RotateAllKeys(true, "test"))
	// Give any (erroneous) in-flight hook a chance to fire before asserting.
	time.Sleep(300 * time.Millisecond)
	require.Zero(t, triggerHits.Load(), "superior must not be notified after jwks_sync is disabled")
}
