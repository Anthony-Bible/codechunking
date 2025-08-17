package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TransactionManager manages database transactions.
type TransactionManager struct {
	pool *pgxpool.Pool
}

// NewTransactionManager creates a new transaction manager.
func NewTransactionManager(pool *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{
		pool: pool,
	}
}

// WithTransaction executes a function within a database transaction.
func (tm *TransactionManager) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create a new context with the transaction
	txCtx := context.WithValue(ctx, txContextKey{}, tx)

	// Execute the function
	err = fn(txCtx)
	if err != nil {
		// Rollback on error
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("failed to rollback transaction after error %w: %w", err, rollbackErr)
		}
		return err
	}

	// Commit the transaction
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return fmt.Errorf("failed to commit transaction: %w", commitErr)
	}

	return nil
}

// WithTransactionIsolation executes a function within a database transaction with specific isolation level.
func (tm *TransactionManager) WithTransactionIsolation(
	ctx context.Context,
	isolation pgx.TxIsoLevel,
	fn func(context.Context) error,
) error {
	txOptions := pgx.TxOptions{IsoLevel: isolation}

	tx, err := tm.pool.BeginTx(ctx, txOptions)
	if err != nil {
		return fmt.Errorf("failed to begin transaction with isolation level %s: %w", isolation, err)
	}

	// Create a new context with the transaction
	txCtx := context.WithValue(ctx, txContextKey{}, tx)

	// Execute the function
	err = fn(txCtx)
	if err != nil {
		// Rollback on error
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("failed to rollback transaction after error %w: %w", err, rollbackErr)
		}
		return err
	}

	// Commit the transaction
	if commitErr2 := tx.Commit(ctx); commitErr2 != nil {
		return fmt.Errorf("failed to commit transaction: %w", commitErr2)
	}

	return nil
}

// WithTransactionRetry executes a function within a database transaction with retry logic for deadlocks.
func (tm *TransactionManager) WithTransactionRetry(
	ctx context.Context,
	maxRetries int,
	fn func(context.Context) error,
) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := tm.WithTransaction(ctx, fn)
		if err == nil {
			return nil
		}

		// Check if it's a deadlock or serialization failure that we can retry
		if isRetryableError(err) && attempt < maxRetries {
			lastErr = err
			// Wait a bit before retrying (exponential backoff could be added here)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Millisecond * time.Duration((attempt+1)*10)):
				continue
			}
		}

		return err
	}

	return lastErr
}

// isRetryableError checks if an error indicates a condition that might be resolved by retrying.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for PostgreSQL deadlock and serialization failure error codes
	retryablePatterns := []string{
		"deadlock detected",
		"could not serialize access",
		"connection reset by peer",
		"connection refused",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

// txContextKey is used as a key for storing transactions in context.
type txContextKey struct{}

// GetTx retrieves a transaction from context, or returns the pool if no transaction exists.
func GetTx(ctx context.Context, pool *pgxpool.Pool) pgx.Tx {
	if tx, ok := ctx.Value(txContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return nil
}

// QueryInterface represents either a connection pool or a transaction.
type QueryInterface interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// GetQueryInterface returns the appropriate query interface (tx or pool).
func GetQueryInterface(ctx context.Context, pool *pgxpool.Pool) QueryInterface {
	if tx := GetTx(ctx, pool); tx != nil {
		return tx
	}
	return pool
}
