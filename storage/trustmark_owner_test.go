package storage

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/url"
	"testing"

	"github.com/go-oidfed/lib/jwx"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/stretchr/testify/require"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// newTrustMarkOwnerTestStorage creates a unique in-memory SQLite database for
// trust mark owner tests.
func newTrustMarkOwnerTestStorage(t *testing.T) *Storage {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", url.PathEscape(t.Name()))
	store, err := NewStorage(
		Config{
			Driver: DriverSQLite,
			DSN:    dsn,
		},
	)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	return store
}

func testOwnerJWKS(t *testing.T) model.JWKS {
	t.Helper()
	sk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pub, err := jwk.PublicKeyOf(sk)
	require.NoError(t, err)
	require.NoError(t, jwk.AssignKeyID(pub))
	set := jwx.NewJWKS()
	require.NoError(t, set.AddKey(pub))
	return model.JWKS{Keys: set}
}

// TestTrustMarkTypesStorage_CreateOwnerReusesDeletedEntityID guards against a
// regression where deleting a trust mark type's owner did not release its
// entity_id for reuse: the row was soft-deleted (invisible to reads) but the
// unique index still blocked re-creation with the same entity_id.
func TestTrustMarkTypesStorage_CreateOwnerReusesDeletedEntityID(t *testing.T) {
	store := newTrustMarkOwnerTestStorage(t)
	typesStore := store.TrustMarkTypesStorage()

	typ, err := typesStore.Create(model.AddTrustMarkType{TrustMarkType: "type-1"})
	require.NoError(t, err)
	typeID := fmt.Sprintf("%d", typ.ID)

	const entityID = "https://mesh-tm-owner.example.org"

	created, err := typesStore.CreateOwner(typeID, model.AddTrustMarkOwner{
		EntityID: entityID,
		JWKS:     testOwnerJWKS(t),
	})
	require.NoError(t, err)
	require.Equal(t, entityID, created.EntityID)

	require.NoError(t, typesStore.DeleteOwner(typeID))

	_, err = typesStore.GetOwner(typeID)
	require.Error(t, err)
	require.IsType(t, model.NotFoundError(""), err)

	recreated, err := typesStore.CreateOwner(typeID, model.AddTrustMarkOwner{
		EntityID: entityID,
		JWKS:     testOwnerJWKS(t),
	})
	require.NoError(t, err, "reusing a deleted owner's entity_id should succeed")
	require.Equal(t, entityID, recreated.EntityID)

	owner, err := typesStore.GetOwner(typeID)
	require.NoError(t, err)
	require.Equal(t, entityID, owner.EntityID)
	require.Equal(t, recreated.ID, owner.ID)
}

// TestTrustMarkTypesStorage_UpdateOwnerRenamesToDeletedEntityID guards against
// the same unique-index regression on the owner rename path.
func TestTrustMarkTypesStorage_UpdateOwnerRenamesToDeletedEntityID(t *testing.T) {
	store := newTrustMarkOwnerTestStorage(t)
	typesStore := store.TrustMarkTypesStorage()

	typ, err := typesStore.Create(model.AddTrustMarkType{TrustMarkType: "type-2"})
	require.NoError(t, err)
	typeID := fmt.Sprintf("%d", typ.ID)

	_, err = typesStore.CreateOwner(typeID, model.AddTrustMarkOwner{
		EntityID: "https://owner-a.example.org",
		JWKS:     testOwnerJWKS(t),
	})
	require.NoError(t, err)
	require.NoError(t, typesStore.DeleteOwner(typeID))

	_, err = typesStore.CreateOwner(typeID, model.AddTrustMarkOwner{
		EntityID: "https://owner-b.example.org",
		JWKS:     testOwnerJWKS(t),
	})
	require.NoError(t, err)

	renamed, err := typesStore.UpdateOwner(typeID, model.AddTrustMarkOwner{
		EntityID: "https://owner-a.example.org",
		JWKS:     testOwnerJWKS(t),
	})
	require.NoError(t, err, "renaming an owner to a deleted entity_id should succeed")
	require.Equal(t, "https://owner-a.example.org", renamed.EntityID)

	owner, err := typesStore.GetOwner(typeID)
	require.NoError(t, err)
	require.Equal(t, "https://owner-a.example.org", owner.EntityID)
}

// TestTrustMarkTypesStorage_CreateOwnerDuplicateEntityID verifies that an
// entity_id held by an active owner still conflicts.
func TestTrustMarkTypesStorage_CreateOwnerDuplicateEntityID(t *testing.T) {
	store := newTrustMarkOwnerTestStorage(t)
	typesStore := store.TrustMarkTypesStorage()

	typ1, err := typesStore.Create(model.AddTrustMarkType{TrustMarkType: "type-1"})
	require.NoError(t, err)
	typ2, err := typesStore.Create(model.AddTrustMarkType{TrustMarkType: "type-2"})
	require.NoError(t, err)

	_, err = typesStore.CreateOwner(fmt.Sprintf("%d", typ1.ID), model.AddTrustMarkOwner{
		EntityID: "https://dup.example.org",
		JWKS:     testOwnerJWKS(t),
	})
	require.NoError(t, err)

	_, err = typesStore.CreateOwner(fmt.Sprintf("%d", typ2.ID), model.AddTrustMarkOwner{
		EntityID: "https://dup.example.org",
		JWKS:     testOwnerJWKS(t),
	})
	require.Error(t, err)
	require.IsType(t, model.AlreadyExistsError(""), err)
}
