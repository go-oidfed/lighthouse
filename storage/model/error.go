package model

import (
	"fmt"
)

// NotFoundError is an error signaling that something was not found in the
// database
type NotFoundError string

// Error implements the error interface
func (e NotFoundError) Error() string {
	return string(e)
}

// NotFoundErrorFmt returns a NotFoundError from the passed format string and parameters
func NotFoundErrorFmt(format string, params ...any) NotFoundError {
	return NotFoundError(fmt.Sprintf(format, params...))
}

// AlreadyExistsError signals a uniqueness/conflict violation
type AlreadyExistsError string

func (e AlreadyExistsError) Error() string { return string(e) }

func AlreadyExistsErrorFmt(format string, params ...any) AlreadyExistsError {
	return AlreadyExistsError(fmt.Sprintf(format, params...))
}
