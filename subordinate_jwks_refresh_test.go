package lighthouse

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/url"
	"testing"

	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/jwx/v4/jws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

func newTestStorage(t *testing.T) *storage.Storage {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", url.PathEscape(t.Name()))
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	return store
}

func rsaKey(t *testing.T) jwx.SigningKey {
	t.Helper()
	sk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return sk
}

func pubJWKS(t *testing.T, sk jwx.SigningKey) jwx.JWKS {
	t.Helper()
	pub, err := jwk.PublicKeyOf(sk)
	require.NoError(t, err)
	require.NoError(t, jwk.AssignKeyID(pub))
	set := jwx.NewJWKS()
	require.NoError(t, set.AddKey(pub))
	return set
}

func jwksWithKid(t *testing.T, kid string) jwx.JWKS {
	t.Helper()
	sk := rsaKey(t)
	pub, err := jwk.PublicKeyOf(sk)
	require.NoError(t, err)
	require.NoError(t, pub.Set(jwk.KeyIDKey, kid))
	set := jwx.NewJWKS()
	require.NoError(t, set.AddKey(pub))
	return set
}

// TestSubordinateJWKSRefreshStorage_Adapter tests the lighthouse adapter that
// implements oidfed.SubordinateJWKSRefreshStorage over the subordinate DB
// storage.
func TestSubordinateJWKSRefreshStorage_Adapter(t *testing.T) {
	store := newTestStorage(t)
	subStore := store.SubordinateStorage()
	eventStore := store.SubordinateEventsStorage()

	sk := rsaKey(t)
	keys := pubJWKS(t, sk)

	require.NoError(t, subStore.Add(model.ExtendedSubordinateInfo{
		JWKS: model.NewJWKS(keys),
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID:         "https://sub-adapter.example",
			Status:           model.StatusActive,
			EnableJWKSUpdate: true,
		},
	}))

	adapter := NewSubordinateJWKSRefreshStorage(subStore, eventStore)

	listed, err := adapter.ListEnabled()
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "https://sub-adapter.example", listed[0].EntityID)
	assert.True(t, listed[0].EnableJWKSUpdate)

	got, err := adapter.Get("https://sub-adapter.example")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "https://sub-adapter.example", got.EntityID)

	missing, err := adapter.Get("https://nope.example")
	require.NoError(t, err)
	assert.Nil(t, missing)

	// UpdateJWKS replaces keys and records an event.
	newKeys := jwksWithKid(t, "new-kid")
	require.NoError(t, adapter.UpdateJWKS("https://sub-adapter.example", newKeys))

	info, err := subStore.Get("https://sub-adapter.example")
	require.NoError(t, err)
	k, _ := info.JWKS.Keys.Key(0)
	kid, _ := k.KeyID()
	assert.Equal(t, "new-kid", kid)

	events, _, err := eventStore.GetBySubordinateID(info.ID, model.EventQueryOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, events)
	assert.Equal(t, model.EventTypeJWKSRefreshed, events[0].Type)
}

// TestJwksUpdate_SignedJWKSetVerification tests approach C's core security
// check: a signed JWK Set signed with a currently-known subordinate key
// verifies, while one signed with an unknown key does not.
func TestJwksUpdate_SignedJWKSetVerification(t *testing.T) {
	subSK := rsaKey(t)
	subKeys := pubJWKS(t, subSK)

	newKeys := jwksWithKid(t, "new-kid")
	signer := jwx.NewSingleKeyVersatileSigner(subSK, jwa.RS256())
	gs := jwx.NewGeneralJWTSigner(signer, []jwa.SignatureAlgorithm{jwa.RS256()})
	headers := jws.NewHeaders()
	subPub, _ := subKeys.Key(0)
	subKID, _ := subPub.KeyID()
	require.NoError(t, headers.Set(jws.KeyIDKey, subKID))
	jwtBytes, err := gs.JWTWithHeaders(
		map[string]any{
			"keys": newKeys,
			"iss":  "https://sub.example",
			"sub":  "https://sub.example",
		},
		headers,
		oidfedconst.JWTTypeJWKS,
	)
	require.NoError(t, err)

	parsed, err := oidfed.ParseSignedJWKS(jwtBytes)
	require.NoError(t, err)
	assert.True(t, parsed.Verify(subKeys), "signature should verify against stored keys")

	otherKeys := pubJWKS(t, rsaKey(t))
	assert.False(t, parsed.Verify(otherKeys), "signature should NOT verify against unknown keys")
}

func TestAddedRemovedMsg(t *testing.T) {
	assert.Equal(t, "", addedRemovedMsg(nil, nil))
	assert.Equal(t, "added: a, b", addedRemovedMsg([]string{"a", "b"}, nil))
	assert.Equal(t, "removed: x", addedRemovedMsg(nil, []string{"x"}))
	assert.Equal(t, "added: a; removed: x", addedRemovedMsg([]string{"a"}, []string{"x"}))
}

func TestBoolPtrIfTrue(t *testing.T) {
	assert.Nil(t, boolPtrIfTrue(false, "msg"))
	p := boolPtrIfTrue(true, "msg")
	require.NotNil(t, p)
	assert.Equal(t, "msg", *p)
}
