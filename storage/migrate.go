package storage

import (
	"github.com/pkg/errors"
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
