package adminapi

import (
	"errors"
	"testing"
)

func TestIsUniqueConstraintError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"NilError", nil, false},
		{"SQLiteUNIQUEConstraintFailed", errors.New("UNIQUE constraint failed: table.column"), true},
		{"SQLiteConstraintFailed", errors.New("constraint failed"), true},
		{"MySQLDuplicateEntry", errors.New("Duplicate entry 'val' for key 'idx'"), true},
		{"MySQLError1062", errors.New("Error 1062: ..."), true},
		{"PostgresDuplicateKeyValue", errors.New("duplicate key value violates unique constraint"), true},
		{"PostgresViolatesUniqueConstraint", errors.New("violates unique constraint \"idx\""), true},
		{"UnrelatedError", errors.New("connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUniqueConstraintError(tt.err)
			if got != tt.want {
				t.Errorf("isUniqueConstraintError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
