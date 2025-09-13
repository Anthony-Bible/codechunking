// Package cmd provides command-line interface functionality for the codechunking application.
/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // File source driver
	_ "github.com/lib/pq"                                // PostgreSQL driver
	"github.com/spf13/cobra"
)

// newMigrateCmd creates and returns the migrate command.
func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration management",
		Long: `Manage database migrations for the codechunking application.

This command provides comprehensive migration management including:
- Applying pending migrations (up)
- Rolling back migrations (down)
- Creating new migration files (create)
- Checking migration status (version)
- Forcing migration version (force)

All operations use the database configuration from config files and environment variables.`,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			// Ensure configuration is loaded
			if GetConfig() == nil {
				slogger.ErrorNoCtx("Configuration not loaded", nil)
				os.Exit(1)
			}
		},
	}

	// Add subcommands
	cmd.AddCommand(newMigrateUpCmd())
	cmd.AddCommand(newMigrateDownCmd())
	cmd.AddCommand(newMigrateCreateCmd())
	cmd.AddCommand(newMigrateVersionCmd())
	cmd.AddCommand(newMigrateForceCmd())

	return cmd
}

// Migration helper functions

// getMigrationInstance creates and returns a migrate instance.
func getMigrationInstance() (*migrate.Migrate, error) {
	cfg := GetConfig()
	if cfg == nil {
		return nil, errors.New("configuration not loaded")
	}

	// Create database connection
	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Test the connection
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create postgres driver
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	// Get absolute path to migrations directory
	migrationsPath, err := filepath.Abs("migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path to migrations: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return m, nil
}

// newMigrateUpCmd creates the migrate up subcommand.
func newMigrateUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		Long:  `Apply all pending database migrations to bring the database schema up to date.`,
		Run: func(_ *cobra.Command, _ []string) {
			slogger.InfoNoCtx("Starting database migration up", nil)

			m, err := getMigrationInstance()
			if err != nil {
				slogger.ErrorNoCtx("Failed to create migration instance", slogger.Fields{"error": err.Error()})
				os.Exit(1)
			}
			defer m.Close()

			if err := m.Up(); err != nil {
				if errors.Is(err, migrate.ErrNoChange) {
					slogger.InfoNoCtx("No pending migrations to apply", nil)
					return
				}
				slogger.ErrorNoCtx("Failed to apply migrations", slogger.Fields{"error": err.Error()})
				os.Exit(1)
			}

			version, dirty, err := m.Version()
			if err != nil {
				slogger.ErrorNoCtx("Failed to get migration version", slogger.Fields{"error": err.Error()})
			} else {
				slogger.InfoNoCtx("Migrations applied successfully", slogger.Fields{
					"version": version,
					"dirty":   dirty,
				})
			}
		},
	}
}

// newMigrateDownCmd creates the migrate down subcommand.
func newMigrateDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [steps]",
		Short: "Rollback migrations",
		Long:  `Rollback database migrations. Specify number of steps or use -1 for all.`,
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			executeMigrationDown(args)
		},
	}
}

// executeMigrationDown handles the migration down logic.
func executeMigrationDown(args []string) {
	steps := parseStepsArgument(args)
	slogger.InfoNoCtx("Starting database migration down", slogger.Fields{"steps": steps})

	m, err := getMigrationInstance()
	if err != nil {
		slogger.ErrorNoCtx("Failed to create migration instance", slogger.Fields{"error": err.Error()})
		os.Exit(1)
	}

	// Handle rollback and cleanup
	if err := performRollback(m, steps); err != nil {
		closeMigrationInstance(m)
		os.Exit(1)
	}

	logMigrationResult(m)
	closeMigrationInstance(m)
}

// performRollback executes the rollback operation.
func performRollback(m *migrate.Migrate, steps int) error {
	rollbackErr := executeRollback(m, steps)
	if rollbackErr == nil {
		return nil
	}

	if errors.Is(rollbackErr, migrate.ErrNoChange) {
		slogger.InfoNoCtx("No migrations to rollback", nil)
		return nil
	}

	slogger.ErrorNoCtx("Failed to rollback migrations", slogger.Fields{"error": rollbackErr.Error()})
	return rollbackErr
}

// closeMigrationInstance safely closes the migration instance.
func closeMigrationInstance(m *migrate.Migrate) {
	if sourceErr, databaseErr := m.Close(); sourceErr != nil || databaseErr != nil {
		if sourceErr != nil {
			slogger.ErrorNoCtx("Failed to close migration source", slogger.Fields{"error": sourceErr.Error()})
		}
		if databaseErr != nil {
			slogger.ErrorNoCtx("Failed to close migration database", slogger.Fields{"error": databaseErr.Error()})
		}
	}
}

// parseStepsArgument parses the steps argument from command line.
func parseStepsArgument(args []string) int {
	steps := 1 // Default to rolling back 1 migration
	if len(args) > 0 {
		var err error
		steps, err = strconv.Atoi(args[0])
		if err != nil {
			slogger.ErrorNoCtx("Invalid steps argument", slogger.Fields{"error": err.Error()})
			os.Exit(1)
		}
	}
	return steps
}

// executeRollback performs the actual rollback operation.
func executeRollback(m *migrate.Migrate, steps int) error {
	if steps == -1 {
		return m.Down()
	}
	return m.Steps(-steps)
}

// logMigrationResult logs the result of a migration operation.
func logMigrationResult(m *migrate.Migrate) {
	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			slogger.InfoNoCtx("All migrations rolled back successfully", nil)
		} else {
			slogger.ErrorNoCtx("Failed to get migration version", slogger.Fields{"error": err.Error()})
		}
	} else {
		slogger.InfoNoCtx("Migrations rolled back successfully", slogger.Fields{
			"version": version,
			"dirty":   dirty,
		})
	}
}

// newMigrateCreateCmd creates the migrate create subcommand.
func newMigrateCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new migration file",
		Long:  `Create a new migration file with the specified name.`,
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			name := args[0]
			slogger.InfoNoCtx("Creating new migration", slogger.Fields{"name": name})

			// This would typically use the migrate CLI tool or implement file creation
			// For now, we'll provide instructions to use the CLI tool
			slogger.InfoNoCtx("Migration creation instructions", slogger.Fields{
				"command": fmt.Sprintf("migrate create -ext sql -dir migrations -seq %s", name),
				"install": "make install-tools",
			})
		},
	}
}

// newMigrateVersionCmd creates the migrate version subcommand.
func newMigrateVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show current migration version",
		Long:  `Display the current migration version and dirty state.`,
		Run: func(_ *cobra.Command, _ []string) {
			m, err := getMigrationInstance()
			if err != nil {
				slogger.ErrorNoCtx("Failed to create migration instance", slogger.Fields{"error": err.Error()})
				os.Exit(1)
			}
			defer m.Close()

			version, dirty, err := m.Version()
			if err != nil {
				if errors.Is(err, migrate.ErrNilVersion) {
					slogger.InfoNoCtx("No migrations have been applied", nil)
					return
				}
				slogger.ErrorNoCtx("Failed to get migration version", slogger.Fields{"error": err.Error()})
				os.Exit(1)
			}

			state := "CLEAN"
			if dirty {
				state = "DIRTY (migration failed, manual intervention required)"
			}
			slogger.InfoNoCtx("Migration version status", slogger.Fields{
				"version": version,
				"state":   state,
			})
		},
	}
}

// newMigrateForceCmd creates the migrate force subcommand.
func newMigrateForceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "force <version>",
		Short: "Force migration version",
		Long: `Force the migration version to a specific value without running migrations.
This is useful for fixing broken migration states.`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			version, err := strconv.Atoi(args[0])
			if err != nil {
				slogger.ErrorNoCtx("Invalid version argument", slogger.Fields{"error": err.Error()})
				os.Exit(1)
			}

			slogger.InfoNoCtx("Forcing migration version", slogger.Fields{"version": version})

			m, err := getMigrationInstance()
			if err != nil {
				slogger.ErrorNoCtx("Failed to create migration instance", slogger.Fields{"error": err.Error()})
				os.Exit(1)
			}
			defer m.Close()

			if err := m.Force(version); err != nil {
				slogger.ErrorNoCtx("Failed to force migration version", slogger.Fields{"error": err.Error()})
				os.Exit(1)
			}

			slogger.InfoNoCtx("Migration version forced successfully", slogger.Fields{"version": version})
		},
	}
}

func init() { //nolint:gochecknoinits // Standard Cobra CLI pattern for command registration
	rootCmd.AddCommand(newMigrateCmd())
}
