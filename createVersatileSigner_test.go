package lighthouse

import (
	"path/filepath"
	"testing"

	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/jwx/keymanagement/public"

	"github.com/go-oidfed/lighthouse/api/adminapi"
	"github.com/go-oidfed/lighthouse/storage"
)

func newDBPKStorage(t *testing.T, typeID string) public.PublicKeyStorage {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "ls_test_"+typeID+".db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	s := storage.NewDBPublicKeyStorage(db, typeID)
	require.NoError(t, s.Load())
	return s
}

// createVersatileSigner merges the KMS-managed and API-managed storages into a
// single published JWKS. A KID must uniquely identify a key, so a key with the
// same kid present in both storages must be published only once.
func TestCreateVersatileSigner_DedupesAcrossStorages(t *testing.T) {
	kms := newDBPKStorage(t, "kms")
	api := newDBPKStorage(t, "api")

	mk := func(kid string) public.PublicKeyEntry {
		_, pk, _, err := jwx.GenerateKeyPair(jwa.RS256(), 2048)
		require.NoError(t, err)
		require.NoError(t, pk.Set("kid", kid))
		return public.PublicKeyEntry{KID: kid, Key: public.JWKKey{Key: pk}}
	}
	shared := "shared-kid"
	onlyAPI := "only-api"
	require.NoError(t, kms.Add(mk(shared)))
	require.NoError(t, api.Add(mk(shared)))
	require.NoError(t, api.Add(mk(onlyAPI)))

	vs, err := createVersatileSigner(adminapi.KeyManagement{
		KMSManagedPKs: kms,
		APIManagedPKs: api,
	})
	require.NoError(t, err)

	jwks, err := vs.JWKS()
	require.NoError(t, err)
	assert.Equal(t, 2, jwks.Len(), "the shared kid must be published once, plus the only-API key")

	kids := make(map[string]bool)
	for _, k := range jwks.All() {
		kid, _ := k.KeyID()
		kids[kid] = true
	}
	assert.Equal(t, map[string]bool{shared: true, onlyAPI: true}, kids)
}
