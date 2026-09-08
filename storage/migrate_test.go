package storage

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TestMigrateDropsStaleSubjectFK simulates a database that still contains the
// stale foreign key constraint on issued_trust_mark_instances and verifies
// migrateIssuedTrustMarkInstanceSubject removes it and restores the indexes.
func TestMigrateDropsStaleSubjectFK(t *testing.T) {
	db := newMigrateTestDB(t, "")
	id := db.Migrator().CurrentDatabase()

	// Create the trust_mark_subjects table first so the FK target exists.
	require.NoError(t, db.Migrator().CreateTable(&model.TrustMarkSubject{}))

	// Manually create issued_trust_mark_instances WITH the stale FK, as an
	// earlier model version would have produced.
	createSQL := `CREATE TABLE issued_trust_mark_instances (
  ` + "`created_at`" + ` integer, ` + "`expires_at`" + ` integer, ` + "`revoked`" + ` numeric,
  ` + "`jti`" + ` text, ` + "`trust_mark_subject_id`" + ` integer, ` + "`updated_at`" + ` integer,
  ` + "`trust_mark_type`" + ` text, ` + "`subject`" + ` text,
  PRIMARY KEY (` + "`jti`" + `),
  CONSTRAINT ` + "`fk_issued_trust_mark_instances_trust_mark_subject`" + `
    FOREIGN KEY (` + "`trust_mark_subject_id`" + `) REFERENCES ` + "`" + id + "_trust_mark_subjects`" + `(` + "`id`" + `)
)`
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS issued_trust_mark_instances").Error)
	require.NoError(t, db.Exec(createSQL).Error)

	require.True(t, db.Migrator().HasConstraint(&model.IssuedTrustMarkInstance{}, issuedTrustMarkInstanceSubjectFK),
		"expected stale FK constraint to exist before migration")

	require.NoError(t, migrateIssuedTrustMarkInstanceSubject(db))

	require.False(t, db.Migrator().HasConstraint(&model.IssuedTrustMarkInstance{}, issuedTrustMarkInstanceSubjectFK),
		"stale FK constraint should be dropped by migration")

	// Indexes must be restored after the SQLite table rebuild.
	for _, idx := range []string{
		"idx_issued_trust_mark_instances_subject",
		"idx_issued_trust_mark_instances_trust_mark_type",
		"idx_issued_trust_mark_instances_trust_mark_subject_id",
		"idx_issued_trust_mark_instances_revoked",
		"idx_issued_trust_mark_instances_expires_at",
	} {
		require.True(t, db.Migrator().HasIndex(&model.IssuedTrustMarkInstance{}, idx), "index %s should exist", idx)
	}

	// A row with trust_mark_subject_id = 0 (no linked subject) must no longer
	// violate any constraint after the migration.
	rec := &model.IssuedTrustMarkInstance{
		JTI:                "test-jti",
		TrustMarkType:      "https://example.com/test",
		Subject:            "https://example.com/entity",
		TrustMarkSubjectID: 0,
	}
	require.NoError(t, db.Create(rec).Error)
}

// newMigrateTestDB opens a temp SQLite DB.
func newMigrateTestDB(t *testing.T, _ string) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "migrate.db") + "?_foreign_keys=on"
	db, err := Connect(Config{Driver: DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	return db
}
