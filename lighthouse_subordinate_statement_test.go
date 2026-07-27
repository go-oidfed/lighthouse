package lighthouse

import (
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/jwx/v4/jws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/unixtime"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// testKeyWithKID generates an ES256 public JWK with the given kid.
func testKeyWithKID(t *testing.T, kid string) jwk.Key {
	t.Helper()
	_, pk, _, err := jwx.GenerateKeyPair(jwa.ES256(), 0)
	require.NoError(t, err)
	require.NoError(t, pk.Set(jwk.KeyIDKey, kid))
	return pk
}

// newMinimalLightHouse builds a LightHouse with just enough wired up for
// CreateSubordinateStatement to work (KV storage + FederationEntity.EntityID).
func newMinimalLightHouse(t *testing.T) *LightHouse {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	return &LightHouse{
		FederationEntity: stubFedEntity{},
		storages: model.Backends{
			KV: store.KeyValue(),
		},
	}
}

type stubFedEntity struct{}

func (stubFedEntity) EntityID() string { return "https://lighthouse.example.org" }
func (stubFedEntity) EntityConfigurationPayload() (*oidfed.EntityStatementPayload, error) {
	return nil, nil
}
func (stubFedEntity) EntityConfigurationJWT() ([]byte, error) { return nil, nil }
func (stubFedEntity) SignEntityStatement(oidfed.EntityStatementPayload) ([]byte, error) {
	return nil, nil
}
func (stubFedEntity) SignEntityStatementWithHeaders(oidfed.EntityStatementPayload, jws.Headers) ([]byte, error) {
	return nil, nil
}

// TestCreateSubordinateStatement_FilterExpiredKeys verifies that expired keys
// are omitted from the published JWKS while non-expired and no-exp keys are
// kept.
func TestCreateSubordinateStatement_FilterExpiredKeys(t *testing.T) {
	now := time.Now()
	fed := newMinimalLightHouse(t)

	jwks := jwx.NewJWKS()
	noExp := testKeyWithKID(t, "no-exp")
	require.NoError(t, jwks.AddKey(noExp))
	future := testKeyWithKID(t, "future")
	require.NoError(t, future.Set("exp", unixtime.Unixtime{Time: now.Add(time.Hour)}))
	require.NoError(t, jwks.AddKey(future))
	past := testKeyWithKID(t, "past")
	require.NoError(t, past.Set("exp", unixtime.Unixtime{Time: now.Add(-time.Hour)}))
	require.NoError(t, jwks.AddKey(past))

	sub := &model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: "https://sub.example.org",
		},
		JWKS: model.JWKS{Keys: jwks},
	}

	payload := fed.CreateSubordinateStatement(sub)

	assert.Equal(t, 2, payload.JWKS.Len(), "expired key should be omitted")
	kids := make(map[string]bool)
	for _, k := range payload.JWKS.All() {
		if kid, ok := k.KeyID(); ok {
			kids[kid] = true
		}
	}
	assert.True(t, kids["no-exp"])
	assert.True(t, kids["future"])
	assert.False(t, kids["past"], "expired key must not appear in published JWKS")
}

// TestCreateSubordinateStatement_ExpCapToMaxKeyExp verifies that the statement
// ExpiresAt is capped to the maximal key expiration when it would otherwise
// outlive all published keys, and that no cap is applied when no published key
// has an exp.
func TestCreateSubordinateStatement_ExpCapToMaxKeyExp(t *testing.T) {
	now := time.Now()
	fed := newMinimalLightHouse(t)

	t.Run("capped to max key exp", func(t *testing.T) {
		jwks := jwx.NewJWKS()
		k := testKeyWithKID(t, "short-lived")
		keyExp := now.Add(5 * time.Minute)
		require.NoError(t, k.Set("exp", unixtime.Unixtime{Time: keyExp}))
		require.NoError(t, jwks.AddKey(k))

		sub := &model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{EntityID: "https://sub-cap.example.org"},
			JWKS:                 model.JWKS{Keys: jwks},
		}
		payload := fed.CreateSubordinateStatement(sub)

		// Default lifetime is 600000s; the statement exp must be capped to ~5m.
		assert.True(t, payload.ExpiresAt.Time.Before(now.Add(storage.DefaultSubordinateStatementLifetime)))
		assert.WithinDuration(t, keyExp, payload.ExpiresAt.Time, 2*time.Second)
	})

	t.Run("no cap when keys have no exp", func(t *testing.T) {
		jwks := jwx.NewJWKS()
		require.NoError(t, jwks.AddKey(testKeyWithKID(t, "no-exp")))

		sub := &model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{EntityID: "https://sub-nocap.example.org"},
			JWKS:                 model.JWKS{Keys: jwks},
		}
		payload := fed.CreateSubordinateStatement(sub)

		assert.WithinDuration(
			t,
			now.Add(storage.DefaultSubordinateStatementLifetime),
			payload.ExpiresAt.Time,
			2*time.Second,
			"exp should remain at now + default lifetime when no keys have exp",
		)
	})
}

// TestCreateSubordinateStatement_AllExpiredEmptyJWKS verifies that when every
// key is expired the published JWKS is empty (and the statement is still
// issued, not errored).
func TestCreateSubordinateStatement_AllExpiredEmptyJWKS(t *testing.T) {
	now := time.Now()
	fed := newMinimalLightHouse(t)

	jwks := jwx.NewJWKS()
	k := testKeyWithKID(t, "expired")
	require.NoError(t, k.Set("exp", unixtime.Unixtime{Time: now.Add(-time.Hour)}))
	require.NoError(t, jwks.AddKey(k))

	sub := &model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{EntityID: "https://sub-empty.example.org"},
		JWKS:                 model.JWKS{Keys: jwks},
	}
	payload := fed.CreateSubordinateStatement(sub)

	assert.Equal(t, 0, payload.JWKS.Len(), "all-expired set should publish empty JWKS")
	assert.Equal(t, "https://sub-empty.example.org", payload.Subject, "statement is still issued")
}
