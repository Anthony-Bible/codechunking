package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// databaseShutdownManager implements DatabaseShutdownManager interface.
type databaseShutdownManager struct {
	databases map[string]DatabaseConnection
	status    []DatabaseConnectionStatus
	metrics   DatabaseShutdownMetrics
	mu        sync.RWMutex
}

// NewDatabaseShutdownManager creates a new DatabaseShutdownManager instance.
func NewDatabaseShutdownManager() DatabaseShutdownManager {
	return &databaseShutdownManager{
		databases: make(map[string]DatabaseConnection),
		status:    make([]DatabaseConnectionStatus, 0),
		metrics: DatabaseShutdownMetrics{
			DatabaseMetrics: make(map[string]DatabaseMetrics),
		},
	}
}

// RegisterDatabase registers a database connection for shutdown management.
func (d *databaseShutdownManager) RegisterDatabase(name string, db DatabaseConnection) error {
	if name == "" {
		return errors.New("database name cannot be empty")
	}
	if db == nil {
		return errors.New("database connection cannot be nil")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.databases[name]; exists {
		return fmt.Errorf("database already registered: %s", name)
	}

	d.databases[name] = db

	// Initialize status
	info := db.GetConnectionInfo()
	status := DatabaseConnectionStatus{
		Name:               name,
		Status:             "active",
		ActiveConnections:  db.GetActiveConnections(),
		MaxConnections:     info.MaxOpenConns,
		ActiveTransactions: db.GetActiveTxns(),
		PendingTxns:        db.GetActiveTxns(),
		LastActivity:       time.Now(),
		IsHealthy:          db.IsHealthy(context.Background()),
	}

	d.status = append(d.status, status)
	d.metrics.TotalDatabases = len(d.databases)

	return nil
}

// DrainConnections gracefully drains active database connections.
func (d *databaseShutdownManager) DrainConnections(ctx context.Context) error {
	d.mu.RLock()
	databases := make(map[string]DatabaseConnection)
	for name, db := range d.databases {
		databases[name] = db
	}
	d.mu.RUnlock()

	for name, db := range databases {
		d.updateDatabaseStatus(name, "draining", "")

		// Set max connections to 0 to prevent new connections
		if err := db.SetMaxConnections(0); err != nil {
			d.updateDatabaseStatus(name, "error", err.Error())
			continue
		}

		// Drain connections with timeout
		drainCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := db.DrainConnections(drainCtx, 10*time.Second); err != nil {
			cancel()
			d.updateDatabaseStatus(name, "error", err.Error())
			d.mu.Lock()
			d.metrics.FailedShutdowns++
			d.mu.Unlock()
			continue
		}
		cancel()

		d.updateDatabaseStatus(name, "drained", "")
	}

	return nil
}

// CommitPendingTransactions attempts to commit all pending transactions.
func (d *databaseShutdownManager) CommitPendingTransactions(ctx context.Context) error {
	d.mu.RLock()
	databases := make(map[string]DatabaseConnection)
	for name, db := range d.databases {
		databases[name] = db
	}
	d.mu.RUnlock()

	for name, db := range databases {
		commitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := db.CommitTxns(commitCtx); err != nil {
			cancel()
			d.updateDatabaseStatus(name, "error", err.Error())
			continue
		}
		cancel()

		// Update metrics
		d.mu.Lock()
		if dbMetrics, exists := d.metrics.DatabaseMetrics[name]; exists {
			dbMetrics.TxnsCommitted++
			d.metrics.DatabaseMetrics[name] = dbMetrics
		}
		d.metrics.TotalTxnsCommitted++
		d.mu.Unlock()
	}

	return nil
}

// RollbackActiveTransactions rolls back active transactions during emergency shutdown.
func (d *databaseShutdownManager) RollbackActiveTransactions(ctx context.Context) error {
	d.mu.RLock()
	databases := make(map[string]DatabaseConnection)
	for name, db := range d.databases {
		databases[name] = db
	}
	d.mu.RUnlock()

	for name, db := range databases {
		rollbackCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := db.RollbackTxns(rollbackCtx); err != nil {
			cancel()
			d.updateDatabaseStatus(name, "error", err.Error())
			continue
		}
		cancel()

		// Update metrics
		d.mu.Lock()
		if dbMetrics, exists := d.metrics.DatabaseMetrics[name]; exists {
			dbMetrics.TxnsRolledBack++
			d.metrics.DatabaseMetrics[name] = dbMetrics
		}
		d.metrics.TotalTxnsRolledBack++
		d.mu.Unlock()
	}

	return nil
}

// CloseAllConnections forcibly closes all database connections.
func (d *databaseShutdownManager) CloseAllConnections(_ context.Context) error {
	d.mu.RLock()
	databases := make(map[string]DatabaseConnection)
	for name, db := range d.databases {
		databases[name] = db
	}
	d.mu.RUnlock()

	for name, db := range databases {
		d.updateDatabaseStatus(name, "closing", "")

		if err := db.Close(); err != nil {
			d.updateDatabaseStatus(name, "error", err.Error())
			d.mu.Lock()
			d.metrics.FailedShutdowns++
			d.mu.Unlock()
			continue
		}

		d.updateDatabaseStatus(name, CircuitBreakerStateClosedStr, "")
		d.mu.Lock()
		d.metrics.SuccessfulShutdowns++
		d.mu.Unlock()
	}

	d.mu.Lock()
	d.metrics.LastShutdownTime = time.Now()
	d.mu.Unlock()

	return nil
}

// GetDatabaseStatus returns the current status of database connections.
func (d *databaseShutdownManager) GetDatabaseStatus() []DatabaseConnectionStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Create a copy to avoid race conditions
	status := make([]DatabaseConnectionStatus, len(d.status))
	copy(status, d.status)
	return status
}

// GetShutdownMetrics returns metrics about database shutdown performance.
func (d *databaseShutdownManager) GetShutdownMetrics() DatabaseShutdownMetrics {
	d.mu.RLock()
	defer d.mu.RUnlock()

	metrics := d.metrics

	// Calculate average drain time
	if d.metrics.SuccessfulShutdowns > 0 {
		totalDrainTime := time.Duration(0)
		count := 0
		for _, dbMetrics := range d.metrics.DatabaseMetrics {
			if dbMetrics.DrainTime > 0 {
				totalDrainTime += dbMetrics.DrainTime
				count++
			}
		}
		if count > 0 {
			metrics.AverageDrainTime = totalDrainTime / time.Duration(count)
		}
	}

	return metrics
}

// updateDatabaseStatus updates the status of a specific database.
func (d *databaseShutdownManager) updateDatabaseStatus(name, status, errorMsg string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, dbStatus := range d.status {
		if dbStatus.Name == name {
			d.updateDatabaseStatusFields(i, status, errorMsg)
			d.handleDrainStartTime(i, status)
			d.handleDrainCompletion(i, name, status)
			d.status[i].LastActivity = time.Now()
			break
		}
	}
}

// updateDatabaseStatusFields updates the basic status fields.
func (d *databaseShutdownManager) updateDatabaseStatusFields(i int, status, errorMsg string) {
	d.status[i].Status = status
	d.status[i].Error = errorMsg
}

// handleDrainStartTime sets the drain start time when draining begins.
func (d *databaseShutdownManager) handleDrainStartTime(i int, status string) {
	if status == ConsumerStatusDraining && d.status[i].DrainStartTime.IsZero() {
		d.status[i].DrainStartTime = time.Now()
	}
}

// handleDrainCompletion handles drain completion and updates metrics.
func (d *databaseShutdownManager) handleDrainCompletion(i int, name, status string) {
	if (status == "drained" || status == CircuitBreakerStateClosedStr) && !d.status[i].DrainStartTime.IsZero() {
		d.status[i].DrainDuration = time.Since(d.status[i].DrainStartTime)
		d.updateDrainMetrics(name, status)
	}
}

// updateDrainMetrics updates database metrics after drain completion.
func (d *databaseShutdownManager) updateDrainMetrics(name, status string) {
	dbMetrics, exists := d.metrics.DatabaseMetrics[name]
	if !exists {
		dbMetrics = DatabaseMetrics{Name: name}
	}
	dbMetrics.DrainTime = d.status[d.findDatabaseStatusIndex(name)].DrainDuration
	dbMetrics.ShutdownAttempts++
	if status != ConsumerStatusError {
		dbMetrics.SuccessfulShutdowns++
	}
	d.metrics.DatabaseMetrics[name] = dbMetrics
}

// findDatabaseStatusIndex finds the index of a database status by name.
func (d *databaseShutdownManager) findDatabaseStatusIndex(name string) int {
	for i, dbStatus := range d.status {
		if dbStatus.Name == name {
			return i
		}
	}
	return -1 // Should not happen in normal operation
}
