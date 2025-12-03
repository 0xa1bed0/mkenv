package state

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/logs"
	_ "modernc.org/sqlite"
)

type Config struct {
	// Path is the absolute path to the sqlite file.
	// Example: /Users/user/.mkenv/state.db
	Path string

	// BusyTimeout is how long another writer waits (in milliseconds)
	// before failing with "database is locked".
	// If zero, defaults to 5000 (5 seconds).
	BusyTimeout int

	// JournalMode, usually "WAL". If empty, defaults to "WAL".
	JournalMode string
}

type DB struct {
	sql *sql.DB
}

var defaultDB *DB

func OpenDefault(ctx context.Context) (*DB, error) {
	if defaultDB == nil {
		dbPath := hostappconfig.StateDBFile()
		logs.Debugf("trying to open state database at %s ...", dbPath)
		var err error
		defaultDB, err = Open(ctx, Config{
			Path: dbPath,
		})
		if err != nil {
			return nil, err
		}
	}

	return defaultDB, nil
}

// Open opens (or creates) the SQLite database, configures
// WAL + busy timeout, and returns a wrapped DB.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("db: Path is required")
	}
	if cfg.BusyTimeout <= 0 {
		cfg.BusyTimeout = 5000
	}
	if cfg.JournalMode == "" {
		cfg.JournalMode = "WAL"
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, fmt.Errorf("db: create dir: %w", err)
	}

	escapedPath := url.PathEscape(cfg.Path)
	dsn := fmt.Sprintf(
		"file:%s?_busy_timeout=%d&_journal_mode=%s&_foreign_keys=ON",
		escapedPath,
		cfg.BusyTimeout,
		url.QueryEscape(cfg.JournalMode),
	)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}

	go func() {
		<-ctx.Done()
		if err := sqlDB.Close(); err != nil {
			logs.Errorf("db close error: %v", err)
		}
	}()

	// Fail early if the DB is not usable.
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(timeoutCtx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return &DB{sql: sqlDB}, nil
}

func (d *DB) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	return d.sql.Close()
}

// Raw exposes the underlying *sql.DB when you really need it.
func (d *DB) Raw() *sql.DB {
	return d.sql
}

// WithTx runs fn inside a transaction. If fn returns an error,
// the transaction is rolled back. Otherwise it is committed.
func (d *DB) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}
	defer func() {
		// Safety net in case fn panics.
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit tx: %w", err)
	}
	return nil
}
