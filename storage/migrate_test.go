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

// legacySubordinateEventsTable is the subordinate_events table as versions
// before entity IDs created it on SQLite: no entity_id column.
const legacySubordinateEventsTable = "CREATE TABLE `subordinate_events` (" +
	"`id` integer PRIMARY KEY AUTOINCREMENT," +
	"`created_at` integer,`updated_at` integer,`deleted_at` datetime," +
	"`subordinate_id` integer,`timestamp` integer,`type` text," +
	"`status` text,`message` text,`actor` text)"

// seedLegacySubordinateEvents creates, in the SQLite file at dsn, the state an
// earlier version leaves behind: a subordinate_events table without
// entity_id, holding events of an active subordinate, of a removed
// (soft-deleted) subordinate whose events the removal soft-deleted, and of a
// subordinate that no longer exists at all.
func seedLegacySubordinateEvents(t *testing.T, dsn string) (activeID, removedID uint) {
	t.Helper()
	db, err := Connect(Config{Driver: DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtendedSubordinateInfo{}))
	require.NoError(t, db.Exec(legacySubordinateEventsTable).Error)
	require.False(t, db.Migrator().HasColumn(&model.SubordinateEvent{}, "entity_id"))

	active := model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{EntityID: "https://active.example.org"},
	}
	removed := model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{EntityID: "https://removed.example.org"},
	}
	require.NoError(t, db.Create(&active).Error)
	require.NoError(t, db.Create(&removed).Error)
	require.NoError(t, db.Delete(&model.ExtendedSubordinateInfo{}, removed.ID).Error)

	insert := "INSERT INTO subordinate_events (subordinate_id, timestamp, type, message, deleted_at) VALUES (?, ?, ?, ?, ?)"
	for _, row := range []struct {
		subordinateID uint
		timestamp     int64
		typ           string
		message       string
		deleted       bool
	}{
		{active.ID, 1, model.EventTypeCreated, "active created", false},
		{removed.ID, 2, model.EventTypeCreated, "removed created", true},
		{removed.ID, 3, model.EventTypeUpdated, "removed updated", true},
		{9999, 4, model.EventTypeCreated, "unknown created", false},
	} {
		var deletedAt any
		if row.deleted {
			deletedAt = "2026-01-01 00:00:00"
		}
		require.NoError(t, db.Exec(insert, row.subordinateID, row.timestamp, row.typ, row.message, deletedAt).Error)
	}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return active.ID, removed.ID
}

// TestMigrateSubordinateEventEntityIDs starts from the schema and data of a
// version before entity IDs and opens it with NewStorage: AutoMigrate adds
// the column and the migration fills it, also on soft-deleted events, without
// losing a row or restoring events that earlier versions soft-deleted.
func TestMigrateSubordinateEventEntityIDs(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "legacy.db")
	activeID, removedID := seedLegacySubordinateEvents(t, dsn)

	store, err := NewStorage(Config{Driver: DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	db := store.DB()
	require.True(t, db.Migrator().HasColumn(&model.SubordinateEvent{}, "entity_id"))

	// Every row is kept, with entity_id filled where the subordinate is known.
	var events []model.SubordinateEvent
	require.NoError(t, db.Unscoped().Order("timestamp ASC").Find(&events).Error)
	require.Len(t, events, 4, "every row is kept")
	want := []struct {
		subordinateID uint
		entityID      string
		message       string
		deleted       bool
	}{
		{activeID, "https://active.example.org", "active created", false},
		{removedID, "https://removed.example.org", "removed created", true},
		{removedID, "https://removed.example.org", "removed updated", true},
		{9999, "", "unknown created", false},
	}
	for i, w := range want {
		require.Equal(t, w.subordinateID, events[i].SubordinateID)
		require.Equal(t, w.entityID, events[i].EntityID)
		require.NotNil(t, events[i].Message)
		require.Equal(t, w.message, *events[i].Message)
		require.Equal(t, w.deleted, events[i].DeletedAt.Valid, "deleted_at of %q is unchanged", w.message)
	}

	// Subordinate rows are not touched: the removed one stays removed.
	var removed model.ExtendedSubordinateInfo
	require.NoError(t, db.Unscoped().First(&removed, removedID).Error)
	require.True(t, removed.DeletedAt.Valid)

	// Events deleted before the upgrade stay deleted: not in the query and
	// not among the removed entities.
	events2 := store.SubordinateEventsStorage()
	entityID := "https://removed.example.org"
	got, total, err := events2.Query(model.EventQueryOpts{EntityID: &entityID})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, got)
	all, total, err := events2.Query(model.EventQueryOpts{})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, all, 2)
	ids, err := events2.RemovedEntityIDs("", 10)
	require.NoError(t, err)
	require.Empty(t, ids)

	// Running it again changes nothing.
	filled, err := migrateSubordinateEventEntityIDs(db)
	require.NoError(t, err)
	require.Zero(t, filled)
	_, err = NewStorage(Config{Driver: DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	var count, deleted int64
	require.NoError(t, db.Unscoped().Model(&model.SubordinateEvent{}).Count(&count).Error)
	require.EqualValues(t, 4, count)
	require.NoError(
		t, db.Unscoped().Model(&model.SubordinateEvent{}).Where("deleted_at IS NOT NULL").Count(&deleted).Error,
	)
	require.EqualValues(t, 2, deleted, "a reboot does not restore soft-deleted events")
}
