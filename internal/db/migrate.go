package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationsDir = "migrations"

// configure points goose at the embedded migrations and the SQLite dialect.
// Safe to call repeatedly.
func configure() error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	return nil
}

// Migrate brings the database up to the latest schema version. Called on app
// startup so a distributed binary self-migrates an older local database.
func Migrate(conn *sql.DB) error {
	if err := configure(); err != nil {
		return err
	}
	if err := goose.Up(conn, migrationsDir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// MigrateTo brings the database up to a specific schema version (dev tooling).
func MigrateTo(conn *sql.DB, version int64) error {
	if err := configure(); err != nil {
		return err
	}
	if err := goose.UpTo(conn, migrationsDir, version); err != nil {
		return fmt.Errorf("goose up to %d: %w", version, err)
	}
	return nil
}

// MigrateDownOne rolls back a single migration (dev tooling).
func MigrateDownOne(conn *sql.DB) error {
	if err := configure(); err != nil {
		return err
	}
	return goose.Down(conn, migrationsDir)
}

// Reset rolls every migration back down to zero (dev tooling — destructive).
func Reset(conn *sql.DB) error {
	if err := configure(); err != nil {
		return err
	}
	return goose.Reset(conn, migrationsDir)
}

// Status prints the migration status table to stdout (dev tooling).
func Status(conn *sql.DB) error {
	if err := configure(); err != nil {
		return err
	}
	return goose.Status(conn, migrationsDir)
}

// Version returns the current schema version.
func Version(conn *sql.DB) (int64, error) {
	if err := configure(); err != nil {
		return 0, err
	}
	return goose.GetDBVersion(conn)
}

// CreateMigration scaffolds a new timestamped SQL migration in dir (dev tooling).
func CreateMigration(dir, name string) error {
	return goose.Create(nil, dir, name, "sql")
}
