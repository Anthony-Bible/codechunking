package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Common error types
var (
	ErrNotFound            = errors.New("record not found")
	ErrAlreadyExists       = errors.New("record already exists")
	ErrForeignKeyViolation = errors.New("foreign key violation")
	ErrConstraintViolation = errors.New("constraint violation")
	ErrConnectionFailed    = errors.New("database connection failed")
	ErrInvalidArgument     = errors.New("invalid argument")
)

// IsNotFoundError checks if an error is a "not found" error
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrNotFound)
}

// IsConstraintViolationError checks if an error is a constraint violation
func IsConstraintViolationError(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// PostgreSQL error codes:
		// 23505: unique_violation
		// 23503: foreign_key_violation
		// 23514: check_violation
		// 23502: not_null_violation
		switch pgErr.Code {
		case "23505", "23503", "23514", "23502":
			return true
		}
	}

	return errors.Is(err, ErrConstraintViolation) || errors.Is(err, ErrAlreadyExists) || errors.Is(err, ErrForeignKeyViolation)
}

// IsConnectionError checks if an error is a connection-related error
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// PostgreSQL connection error codes
		switch pgErr.Code[:2] {
		case "08": // Connection exception
			return true
		case "57": // Operator intervention (includes connection errors)
			return true
		}
	}

	return errors.Is(err, ErrConnectionFailed)
}

// WrapError wraps a database error with appropriate context
func WrapError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Check for context errors first
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s failed: %w", operation, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s failed: %w", operation, context.DeadlineExceeded)
	}

	// Check for invalid argument errors
	if errors.Is(err, ErrInvalidArgument) {
		return fmt.Errorf("%s failed: %w", operation, ErrInvalidArgument)
	}

	if IsNotFoundError(err) {
		return fmt.Errorf("%s failed: %w", operation, ErrNotFound)
	}

	if IsConstraintViolationError(err) {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation
				// Create a constraint error that preserves the original pgErr
				return &ConstraintError{
					Operation:      operation,
					ConstraintType: "unique",
					ConstraintName: pgErr.ConstraintName,
					OriginalError:  pgErr,
				}
			case "23503": // foreign_key_violation
				return &ConstraintError{
					Operation:      operation,
					ConstraintType: "foreign_key",
					ConstraintName: pgErr.ConstraintName,
					OriginalError:  pgErr,
				}
			}
		}
		return fmt.Errorf("%s failed: %w", operation, ErrConstraintViolation)
	}

	if IsConnectionError(err) {
		return fmt.Errorf("%s failed: %w", operation, ErrConnectionFailed)
	}

	return fmt.Errorf("%s failed: %w", operation, err)
}

// ConstraintError represents a database constraint violation error
type ConstraintError struct {
	Operation      string
	ConstraintType string
	ConstraintName string
	OriginalError  error
}

func (e *ConstraintError) Error() string {
	if e.ConstraintType == "unique" {
		// Provide context-aware error messages for better user experience
		if e.ConstraintName == "repositories_normalized_url_key" || e.ConstraintName == "repositories_url_key" {
			return fmt.Sprintf("%s failed: duplicate repository detected", e.Operation)
		}
		return fmt.Sprintf("%s failed: record already exists", e.Operation)
	}
	if e.ConstraintType == "foreign_key" {
		return fmt.Sprintf("%s failed: foreign key violation", e.Operation)
	}
	return fmt.Sprintf("%s failed: constraint violation", e.Operation)
}

func (e *ConstraintError) Unwrap() error {
	return e.OriginalError
}

func (e *ConstraintError) Is(target error) bool {
	switch e.ConstraintType {
	case "unique":
		return errors.Is(target, ErrAlreadyExists)
	case "foreign_key":
		return errors.Is(target, ErrForeignKeyViolation)
	default:
		return errors.Is(target, ErrConstraintViolation)
	}
}

// RepositoryError represents a repository-specific error
type RepositoryError struct {
	Operation string
	Err       error
}

func (e *RepositoryError) Error() string {
	return fmt.Sprintf("repository operation '%s' failed: %v", e.Operation, e.Err)
}

func (e *RepositoryError) Unwrap() error {
	return e.Err
}

// NewRepositoryError creates a new repository error
func NewRepositoryError(operation string, err error) *RepositoryError {
	return &RepositoryError{
		Operation: operation,
		Err:       WrapError(err, operation),
	}
}
