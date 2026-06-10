package storage

import (
	"github.com/go-oidfed/lib/jwx/keymanagement/public"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// DBPEMStorer implements kms.PEMStorer using a database backend.
type DBPEMStorer struct {
	db  *gorm.DB
	tbl string
}

// NewDBPEMStorer creates a new DBPEMStorer.
func NewDBPEMStorer(db *gorm.DB, typeID string) *DBPEMStorer {
	tableName := "private_keys"
	if typeID != "" {
		tableName = tableName + "_" + typeID
	}
	d := &DBPEMStorer{
		db:  db,
		tbl: tableName,
	}
	// Initialize the table
	if err := d.Load(); err != nil {
		panic(err)
	}
	return d
}

// Load initializes the database table.
func (d *DBPEMStorer) Load() error {
	return d.db.Table(d.tbl).AutoMigrate(&model.PrivateKeyEntry{})
}

// ReadPEM retrieves a PEM-encoded private key by KID.
func (d *DBPEMStorer) ReadPEM(kid string) ([]byte, error) {
	var row model.PrivateKeyEntry
	err := d.db.Session(&gorm.Session{NewDB: true}).Table(d.tbl).Where("kid = ?", kid).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, public.NotFoundError{KID: kid}
		}
		return nil, errors.WithStack(err)
	}
	return []byte(row.PEMData), nil
}

// WritePEM stores or updates a PEM-encoded private key by KID.
func (d *DBPEMStorer) WritePEM(kid string, data []byte) error {
	entry := model.PrivateKeyEntry{
		KID:     kid,
		PEMData: model.PEMData(data),
	}
	return errors.WithStack(
		d.db.Session(&gorm.Session{NewDB: true}).Table(d.tbl).Clauses(
			clause.OnConflict{
				Columns: []clause.Column{
					{Name: "kid"},
				},
				DoUpdates: clause.AssignmentColumns(
					[]string{
						"pem_data",
						"updated_at",
					},
				),
			},
		).Create(&entry).Error,
	)
}

// Delete removes a private key by KID.
func (d *DBPEMStorer) Delete(kid string) error {
	return errors.WithStack(
		d.db.Session(&gorm.Session{NewDB: true}).Table(d.tbl).Where("kid = ?", kid).Delete(&model.PrivateKeyEntry{}).Error,
	)
}

// GetAll returns all stored private keys (for debugging/migration purposes).
func (d *DBPEMStorer) GetAll() ([]model.PrivateKeyEntry, error) {
	var rows []model.PrivateKeyEntry
	err := errors.WithStack(d.db.Session(&gorm.Session{NewDB: true}).Table(d.tbl).Find(&rows).Error)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return rows, err
}
