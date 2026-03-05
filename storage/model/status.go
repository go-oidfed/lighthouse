package model

import (
	"fmt"
)

// Status is a type for holding a status for something that is stored in the
// database; this type describes the status or state of the entity,
// e.g. "blocked" or "active"
type Status int

// Constants for Status
const (
	StatusActive Status = iota
	StatusBlocked
	StatusPending
	StatusInactive
)

// String returns the canonical string representation for the status.
func (s Status) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusBlocked:
		return "blocked"
	case StatusPending:
		return "pending"
	case StatusInactive:
		return "inactive"
	default:
		return "unknown"
	}
}

// Valid reports whether the status is one of the defined constants.
func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusBlocked, StatusPending, StatusInactive:
		return true
	default:
		return false
	}
}

// MarshalJSON encodes the status as a JSON string.
func (s Status) MarshalJSON() ([]byte, error) {
	// Unknown maps to "unknown" to avoid failing marshaling; consumers should validate.
	return []byte("\"" + s.String() + "\""), nil
}

// UnmarshalJSON decodes the status from a JSON string.
func (s *Status) UnmarshalJSON(b []byte) error {
	// Expect a quoted string
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("status must be a JSON string")
	}
	val := string(b[1 : len(b)-1])
	ps, err := ParseStatus(val)
	if err != nil {
		return err
	}
	*s = ps
	return nil
}

// ParseStatus converts a string to a Status, returning an error for invalid values.
func ParseStatus(v string) (Status, error) {
	switch v {
	case "active":
		return StatusActive, nil
	case "blocked":
		return StatusBlocked, nil
	case "pending":
		return StatusPending, nil
	case "inactive":
		return StatusInactive, nil
	}
	return 0, fmt.Errorf("invalid status: %s", v)
}
