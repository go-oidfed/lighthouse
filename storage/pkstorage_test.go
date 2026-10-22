package storage

import (
	"path/filepath"
	"testing"

	"github.com/go-oidfed/lib/jwx/keymanagement/public"
	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/go-oidfed/lib/jwx"
)

// newSQLitePKStorage creates an isolated SQLite-backed DBPublicKeyStorage.
func newSQLitePKStorage(t *testing.T) *DBPublicKeyStorage {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "pk_test.db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	s := NewDBPublicKeyStorage(db, "testtype")
	require.NoError(t, s.Load())
	return s
}

func newDBPK(t *testing.T, kid string) public.PublicKeyEntry {
	t.Helper()
	_, pk, _, err := jwx.GenerateKeyPair(jwa.RS256(), 2048)
	require.NoError(t, err)
	require.NoError(t, pk.Set("kid", kid))
	return public.PublicKeyEntry{KID: kid, Key: public.JWKKey{Key: pk}}
}

func TestDBPublicKeyStorage_AddRejectsDuplicateKID(t *testing.T) {
	s := newSQLitePKStorage(t)
	kid := "dup-kid"
	require.NoError(t, s.Add(newDBPK(t, kid)))

	err := s.Add(newDBPK(t, kid))
	require.Error(t, err)
	var dup public.DuplicateKIDError
	require.ErrorAs(t, err, &dup)
	assert.Equal(t, kid, dup.KID)
}

func TestDBPublicKeyStorage_AddAllIsAtomic(t *testing.T) {
	t.Run("existing kid rejected and nothing added", func(t *testing.T) {
		s := newSQLitePKStorage(t)
		kid := "existing"
		require.NoError(t, s.Add(newDBPK(t, kid)))

		err := s.AddAll([]public.PublicKeyEntry{newDBPK(t, "a"), newDBPK(t, kid)})
		require.Error(t, err)
		var dup public.DuplicateKIDError
		require.ErrorAs(t, err, &dup)
		assert.Equal(t, kid, dup.KID)

		got, gErr := s.Get("a")
		require.NoError(t, gErr)
		assert.Nil(t, got, "nothing from the conflicting batch must be added")
	})

	t.Run("intra-batch duplicate rejected and nothing added", func(t *testing.T) {
		s := newSQLitePKStorage(t)
		kid := "intra-dup"
		err := s.AddAll([]public.PublicKeyEntry{newDBPK(t, kid), newDBPK(t, kid)})
		require.Error(t, err)
		var dup public.DuplicateKIDError
		require.ErrorAs(t, err, &dup)
		assert.Equal(t, kid, dup.KID)

		got, gErr := s.Get(kid)
		require.NoError(t, gErr)
		assert.Nil(t, got, "nothing from the conflicting batch must be added")
	})

	t.Run("clean batch adds all", func(t *testing.T) {
		s := newSQLitePKStorage(t)
		require.NoError(t, s.AddAll([]public.PublicKeyEntry{newDBPK(t, "x"), newDBPK(t, "y")}))
		x, xErr := s.Get("x")
		require.NoError(t, xErr)
		require.NotNil(t, x)
		y, yErr := s.Get("y")
		require.NoError(t, yErr)
		require.NotNil(t, y)
	})
}
