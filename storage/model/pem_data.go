package model

import (
	"database/sql/driver"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// PEMData stores PEM-encoded private key bytes with dialect-specific migration support.
type PEMData []byte

// GormDataType declares a generic logical type for GORM.
func (PEMData) GormDataType() string {
	return "bytes"
}

// GormDBDataType maps PEMData to a database-specific column type.
func (PEMData) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "mysql":
		return "longblob"
	case "postgres":
		return "bytea"
	case "sqlite":
		return "blob"
	default:
		return "blob"
	}
}

// Value implements driver.Valuer so PEMData can be stored by database drivers.
func (p PEMData) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return []byte(p), nil
}

// Scan implements sql.Scanner so PEMData can be read from database drivers.
func (p *PEMData) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*p = nil
		return nil
	case []byte:
		*p = append((*p)[:0], v...)
		return nil
	case string:
		*p = append((*p)[:0], v...)
		return nil
	default:
		return fmt.Errorf("unsupported PEMData scan type %T", value)
	}
}
