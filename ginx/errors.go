package ginx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// CustomError represents an HTTP-aware error with a message, error code, and HTTP status.
type CustomError struct {
	Message    string
	ErrorCode  int
	HTTPStatus int
}

// Error implements the error interface.
func (e CustomError) Error() string {
	return e.Message
}

// IsErrNotFound reports whether the error is a GORM record-not-found error.
func IsErrNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// IsErrDuplicateKey reports whether the error is a PostgreSQL unique violation (code 23505).
func IsErrDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// NewCustomError creates a new CustomError with the given message, error code, and HTTP status.
func NewCustomError(message string, errorCode int, httpStatus int) *CustomError {
	return &CustomError{
		Message:    message,
		ErrorCode:  errorCode,
		HTTPStatus: httpStatus,
	}
}

// Commonly used error codes.
const (
	ErrCodeBadRequest          = 40000
	ErrCodeUnauthorized        = 40100
	ErrCodeForbidden           = 40300
	ErrCodeNotFound            = 40400
	ErrCodeRequestTimeout      = 40800
	ErrCodeTooManyRequests     = 42900
	ErrCodeConflict            = 40900
	ErrCodeInternalServerError = 50000
)

// Predefined errors for common HTTP error responses.
var (
	ErrBadRequest      = NewCustomError("Bad request", ErrCodeBadRequest, http.StatusBadRequest)
	ErrUnauthorized    = NewCustomError("Unauthorized", ErrCodeUnauthorized, http.StatusUnauthorized)
	ErrForbidden       = NewCustomError("Forbidden", ErrCodeForbidden, http.StatusForbidden)
	ErrNotFound        = NewCustomError("Not found", ErrCodeNotFound, http.StatusNotFound)
	ErrTooManyRequests = NewCustomError("Too many requests", ErrCodeTooManyRequests, http.StatusTooManyRequests)
	ErrInternalServer  = NewCustomError("Internal server error", ErrCodeInternalServerError, http.StatusInternalServerError)
	ErrRequestTimeout  = NewCustomError("Request timed out", ErrCodeRequestTimeout, http.StatusRequestTimeout)
)

// NewBadRequestError creates a 400 Bad Request error with a custom message.
func NewBadRequestError(message string) *CustomError {
	return NewCustomError(message, ErrCodeBadRequest, http.StatusBadRequest)
}

// NewConflictError creates a 409 Conflict error with a custom message.
func NewConflictError(message string) *CustomError {
	return NewCustomError(message, ErrCodeConflict, http.StatusConflict)
}

// NewUnauthorizedError creates a 401 Unauthorized error with a custom message.
func NewUnauthorizedError(message string) *CustomError {
	return NewCustomError(message, ErrCodeUnauthorized, http.StatusUnauthorized)
}

// NewForbiddenError creates a 403 Forbidden error with a custom message.
func NewForbiddenError(message string) *CustomError {
	return NewCustomError(message, ErrCodeForbidden, http.StatusForbidden)
}

// NewNotFoundError creates a 404 Not Found error with a custom message.
func NewNotFoundError(message string) *CustomError {
	return NewCustomError(message, ErrCodeNotFound, http.StatusNotFound)
}

// NewTooManyRequestsError creates a 429 Too Many Requests error with a custom message.
func NewTooManyRequestsError(message string) *CustomError {
	return NewCustomError(message, ErrCodeTooManyRequests, http.StatusTooManyRequests)
}

// NewInternalServerError creates a 500 Internal Server Error with a custom message.
func NewInternalServerError(message string) *CustomError {
	return NewCustomError(message, ErrCodeInternalServerError, http.StatusInternalServerError)
}

// NewRequestTimeoutError creates a 408 Request Timeout error with a custom message.
func NewRequestTimeoutError(message string) *CustomError {
	return NewCustomError(message, ErrCodeRequestTimeout, http.StatusRequestTimeout)
}

// HandleDBError translates database-related errors into user-friendly CustomErrors.
// It handles context cancellation, GORM errors, and PostgreSQL-specific errors.
func HandleDBError(err error) error {
	// Check for context-related errors
	if errors.Is(err, context.Canceled) {
		return NewRequestTimeoutError("Request was canceled by the client")
	} else if errors.Is(err, context.DeadlineExceeded) {
		return NewRequestTimeoutError("Request timed out")
	}

	// Check for GORM-specific errors
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return NewBadRequestError("Duplicate record: the entry already exists")
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return NewNotFoundError("Record not found")
	} else if errors.Is(err, gorm.ErrInvalidTransaction) {
		return NewInternalServerError("Invalid database transaction")
	}

	// Check for PostgreSQL-specific errors
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "23505": // Unique violation
			return NewBadRequestError("Duplicate entry: a record with the same key already exists")
		case "23503": // Foreign key violation
			return NewBadRequestError("Invalid reference: related record not found")
		case "22001": // String data, right truncation
			return NewBadRequestError("Data too long for the field")
		default:
			return NewInternalServerError(fmt.Sprintf("Database error: %s", pgError.Message))
		}
	}

	// Catch other unhandled errors
	if strings.Contains(err.Error(), "deadlock") {
		return NewInternalServerError("Database deadlock detected, please try again")
	}

	// Default case: return original error
	return err
}
