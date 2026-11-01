package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const (
	pingAttempts = 5
	pingTimeout  = 2 * time.Second
	retryDelay   = time.Second

	migrationTimeout = 15 * time.Second
)

// Open creates a PostgreSQL connection pool and waits for the database to
// become ready. Every ping has its own timeout and the number of retries is
// deliberately limited so a bad configuration cannot block startup forever.
func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, errors.New("database URL is empty")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	var pingErr error
	for attempt := 1; attempt <= pingAttempts; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
		pingErr = db.PingContext(pingCtx)
		cancel()

		if pingErr == nil {
			return db, nil
		}
		if attempt == pingAttempts {
			break
		}

		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			db.Close()
			return nil, fmt.Errorf("wait for database: %w", ctx.Err())
		case <-timer.C:
		}
	}

	db.Close()
	return nil, fmt.Errorf("ping database after %d attempts: %w", pingAttempts, pingErr)
}

// Migrate executes the idempotent SQL files in name order, in one transaction.
func Migrate(db *sql.DB, directory string) error {
	if db == nil {
		return errors.New("migrate: database is nil")
	}

	files, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}
		script, err := os.ReadFile(filepath.Join(directory, file.Name()))
		if err != nil {
			return fmt.Errorf("read migration %q: %w", file.Name(), err)
		}
		if strings.TrimSpace(string(script)) == "" {
			return fmt.Errorf("migration %q is empty", file.Name())
		}
		if _, err := tx.ExecContext(ctx, string(script)); err != nil {
			return fmt.Errorf("execute migration %q: %w", file.Name(), err)
		}
		count++
	}
	if count == 0 {
		return errors.New("no SQL migrations found")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}

	return nil
}
