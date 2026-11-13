package storage

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// issuedTrustMarkInstanceSubjectFK is the stale foreign key constraint on the
// issued_trust_mark_instances.trust_mark_subject_id column. It was created by an
// earlier version of the IssuedTrustMarkInstance model that declared a GORM
// TrustMarkSubject association field. TrustMarkSubjectID is a plain, optional
// reference, so a value of 0 must be valid; the FK makes inserting 0 fail for
// subjects that were not pre-registered. AutoMigrate never drops existing
// constraints, so the stale FK must be removed explicitly.
const issuedTrustMarkInstanceSubjectFK = "fk_issued_trust_mark_instances_trust_mark_subject"

// migrateIssuedTrustMarkInstanceSubject drops the stale foreign key constraint
// on issued_trust_mark_instances.trust_mark_subject_id if it exists. It is
// idempotent and safe to run on every boot regardless of the database driver.
//
// On SQLite the constraint is a table-level clause, so dropping it rebuilds the
// table (which also removes the model's indexes); a follow-up AutoMigrate for
// the model restores any missing indexes so the schema converges in one boot.
// On Postgres/MySQL DropConstraint issues ALTER TABLE and the follow-up
// AutoMigrate is a no-op for the still-present indexes.
func migrateIssuedTrustMarkInstanceSubject(db *gorm.DB) error {
	if !db.Migrator().HasConstraint(&model.IssuedTrustMarkInstance{}, issuedTrustMarkInstanceSubjectFK) {
		return nil
	}

	if err := db.Migrator().DropConstraint(&model.IssuedTrustMarkInstance{}, issuedTrustMarkInstanceSubjectFK); err != nil {
		return errors.Wrap(err, "migration: drop issued_trust_mark_instances subject foreign key failed")
	}

	if err := db.AutoMigrate(&model.IssuedTrustMarkInstance{}); err != nil {
		return errors.Wrap(err, "migration: restore issued_trust_mark_instances schema failed")
	}

	return nil
}

// migrateSubordinateEventEntityIDs brings subordinate events written by
// earlier versions in line with events keyed by entity ID. AutoMigrate adds
// the entity_id column; this migration then fills entity_id of events that
// have none from their subordinate row, including a removed (soft-deleted)
// subordinate. Events of subordinates that no longer exist at all keep an
// empty entity_id. Soft-deleted events get an entity_id too but stay
// deleted: events that earlier versions deleted with their subordinate are
// not restored. Subordinate rows are never touched.
//
// It only changes rows with an empty entity_id, so it is idempotent and safe
// to run on every boot regardless of the database driver.
func migrateSubordinateEventEntityIDs(db *gorm.DB) (filled int64, err error) {
	stmt := &gorm.Statement{DB: db}
	if err = stmt.Parse(&model.SubordinateEvent{}); err != nil {
		return 0, errors.Wrap(err, "migration: parse subordinate_events schema failed")
	}
	eventsTable := stmt.Schema.Table
	subordinatesTable := model.ExtendedSubordinateInfo{}.TableName()
	subordinateEntityID := db.Unscoped().Model(&model.ExtendedSubordinateInfo{}).
		Select(subordinatesTable + ".entity_id").
		Where(subordinatesTable + ".id = " + eventsTable + ".subordinate_id")

	result := db.Unscoped().Model(&model.SubordinateEvent{}).
		Where("(entity_id = '' OR entity_id IS NULL) AND subordinate_id <> 0").
		Where("EXISTS (?)", subordinateEntityID).
		UpdateColumn("entity_id", subordinateEntityID)
	if result.Error != nil {
		return 0, errors.Wrap(result.Error, "migration: fill subordinate_events entity_id failed")
	}
	filled = result.RowsAffected

	if filled > 0 {
		log.Info().
			Int64("filled", filled).
			Msg("migration: subordinate events keyed by entity ID")
	}
	return filled, nil
}
