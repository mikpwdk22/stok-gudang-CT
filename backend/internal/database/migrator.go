package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrationRunner handles running numbered SQL migrations
type MigrationRunner struct {
	pool           *pgxpool.Pool
	migrationsPath string
}

// NewMigrationRunner creates a new MigrationRunner
func NewMigrationRunner(pool *pgxpool.Pool, migrationsPath string) *MigrationRunner {
	return &MigrationRunner{
		pool:           pool,
		migrationsPath: migrationsPath,
	}
}

// EnsureMigrationTable creates the schema_migrations tracking table if it doesn't exist
func (m *MigrationRunner) EnsureMigrationTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(50) PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err := m.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}
	return nil
}

// Up applies all pending .up.sql migrations in order
func (m *MigrationRunner) Up(ctx context.Context) error {
	if err := m.EnsureMigrationTable(ctx); err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(m.migrationsPath, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("failed to list migration files: %w", err)
	}

	sort.Strings(files)

	for _, file := range files {
		base := filepath.Base(file)
		// Extract version (e.g., "000001" from "000001_create_users_table.up.sql")
		parts := strings.SplitN(base, "_", 2)
		if len(parts) == 0 {
			continue
		}
		version := parts[0]

		// Check if already applied
		var exists bool
		checkQuery := "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)"
		err := m.pool.QueryRow(ctx, checkQuery, version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", version, err)
		}

		if exists {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		// Execute in transaction
		tx, err := m.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", base, err)
		}

		if len(strings.TrimSpace(string(content))) > 0 {
			if _, err := tx.Exec(ctx, string(content)); err != nil {
				_ = tx.Rollback(ctx)
				return fmt.Errorf("failed to execute migration %s: %w", base, err)
			}
		}

		// Record migration
		recordQuery := "INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)"
		if _, err := tx.Exec(ctx, recordQuery, version, time.Now()); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", base, err)
		}

		log.Printf("Applied migration: %s\n", base)
	}

	return nil
}

