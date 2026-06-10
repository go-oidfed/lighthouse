package model

import (
	"bytes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	"testing"
)

type fakeDialector string

func (d fakeDialector) Name() string                                   { return string(d) }
func (fakeDialector) Initialize(*gorm.DB) error                      { return nil }
func (fakeDialector) Migrator(*gorm.DB) gorm.Migrator                { return nil }
func (fakeDialector) DataTypeOf(*schema.Field) string                { return "" }
func (fakeDialector) DefaultValueOf(*schema.Field) clause.Expression { return nil }
func (fakeDialector) BindVarTo(clause.Writer, *gorm.Statement, any)  {}
func (fakeDialector) QuoteTo(clause.Writer, string)                  {}
func (fakeDialector) Explain(string, ...any) string                  { return "" }

func TestPEMDataColumnType(t *testing.T) {
	tests := []struct {
		dialect string
		want    string
	}{
		{dialect: "mysql", want: "longblob"},
		{dialect: "postgres", want: "bytea"},
		{dialect: "sqlite", want: "blob"},
		{dialect: "other", want: "blob"},
	}

	for _, tt := range tests {
		db, err := gorm.Open(fakeDialector(tt.dialect), &gorm.Config{SkipDefaultTransaction: true})
		if err != nil {
			t.Fatalf("gorm.Open(%q) returned error: %v", tt.dialect, err)
		}
		if got := (PEMData{}).GormDBDataType(db, nil); got != tt.want {
			t.Fatalf("GormDBDataType(%q) = %q, want %q", tt.dialect, got, tt.want)
		}
	}
}

func TestPEMDataScanAndValue(t *testing.T) {
	var p PEMData

	if err := p.Scan([]byte("abc")); err != nil {
		t.Fatalf("Scan([]byte) returned error: %v", err)
	}
	if !bytes.Equal([]byte(p), []byte("abc")) {
		t.Fatalf("Scan([]byte) = %q, want %q", []byte(p), []byte("abc"))
	}

	val, err := p.Value()
	if err != nil {
		t.Fatalf("Value() returned error: %v", err)
	}
	if got, ok := val.([]byte); !ok || !bytes.Equal(got, []byte("abc")) {
		t.Fatalf("Value() = %#v, want []byte(%q)", val, "abc")
	}

	if err := p.Scan("xyz"); err != nil {
		t.Fatalf("Scan(string) returned error: %v", err)
	}
	if !bytes.Equal([]byte(p), []byte("xyz")) {
		t.Fatalf("Scan(string) = %q, want %q", []byte(p), []byte("xyz"))
	}

	if err := p.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) returned error: %v", err)
	}
	if p != nil {
		t.Fatalf("Scan(nil) = %#v, want nil", []byte(p))
	}
}
