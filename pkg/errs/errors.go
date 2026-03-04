// Package errs provides typed application errors with HTTP status codes.
package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// Code represents a machine-readable error code.
type Code string

const (
	CodeBadRequest   Code = "BAD_REQUEST"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeNotFound     Code = "NOT_FOUND"
	CodeConflict     Code = "CONFLICT"
	CodeInternal     Code = "INTERNAL_ERROR"
)

// Error is a typed application error.
type Error struct {
	code    Code
	message string
	cause   error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.cause
}

// NewBadRequest creates a BAD_REQUEST error.
func NewBadRequest(message string) *Error {
	return &Error{code: CodeBadRequest, message: message}
}

// NewUnauthorized creates an UNAUTHORIZED error.
func NewUnauthorized(message string) *Error {
	return &Error{code: CodeUnauthorized, message: message}
}

// NewForbidden creates a FORBIDDEN error.
func NewForbidden(message string) *Error {
	return &Error{code: CodeForbidden, message: message}
}

// NewNotFound creates a NOT_FOUND error.
func NewNotFound(message string) *Error {
	return &Error{code: CodeNotFound, message: message}
}

// NewConflict creates a CONFLICT error.
func NewConflict(message string) *Error {
	return &Error{code: CodeConflict, message: message}
}

// NewInternal creates an INTERNAL_ERROR error.
func NewInternal(message string) *Error {
	return &Error{code: CodeInternal, message: message}
}

// Wrap wraps an existing error with a typed application error.
func Wrap(err error, code Code, message string) *Error {
	return &Error{code: code, message: message, cause: err}
}

// GetCode extracts the error Code from an error. Returns CodeInternal for unknown errors.
func GetCode(err error) Code {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.code
	}
	return CodeInternal
}

// GetMessage extracts the user-facing message from an error.
func GetMessage(err error) string {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.message
	}
	return "internal error"
}

// HTTPStatus returns the HTTP status code for an error.
func HTTPStatus(err error) int {
	switch GetCode(err) {
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
