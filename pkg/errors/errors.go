package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents an error code
type ErrorCode string

const (
	CodeInternal     ErrorCode = "INTERNAL_ERROR"
	CodeNotFound     ErrorCode = "NOT_FOUND"
	CodeInvalidInput ErrorCode = "INVALID_INPUT"
	CodeUnauthorized ErrorCode = "UNAUTHORIZED"
	CodeForbidden    ErrorCode = "FORBIDDEN"
)

// AppError represents an application error
type AppError struct {
	Code    ErrorCode
	Message string
	Status  int
	Err     error
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// New creates a new application error
func New(code ErrorCode, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

// Wrap wraps an existing error
func Wrap(code ErrorCode, message string, status int, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
		Err:     err,
	}
}

// Common error constructors
var (
	ErrInternal     = New(CodeInternal, "Internal server error", http.StatusInternalServerError)
	ErrNotFound     = New(CodeNotFound, "Resource not found", http.StatusNotFound)
	ErrInvalidInput = New(CodeInvalidInput, "Invalid input", http.StatusBadRequest)
	ErrUnauthorized = New(CodeUnauthorized, "Unauthorized", http.StatusUnauthorized)
	ErrForbidden    = New(CodeForbidden, "Forbidden", http.StatusForbidden)
)

