// Package storage provides fast, embedded database backends.
// SQLite: Reliable relational storage for structured data.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNoRows = sql.ErrNoRows
)

// SQLiteStore is a fast SQLite database wrapper
type SQLiteStore struct {
	db     *sql.DB
	path   string
	mu     sync.RWMutex
	closed bool
	logger *slog.Logger
	stmts  map[string]*sql.Stmt
	stmtMu sync.RWMutex
}

// SQLiteOptions configures the SQLite store
type SQLiteOptions struct {
	Path        string
	InMemory    bool
	WALMode     bool
	BusyTimeout time.Duration
	MaxConns    int
	Logger      *slog.Logger
}

// DefaultSQLiteOptions returns optimized defaults
func DefaultSQLiteOptions(path string) SQLiteOptions {
	return SQLiteOptions{
		Path:        path,
		InMemory:    false,
		WALMode:     true,
		BusyTimeout: 5 * time.Second,
		MaxConns:    10,
	}
}

// NewSQLiteStore creates a new SQLite store with optimized settings
func NewSQLiteStore(opts SQLiteOptions) (*SQLiteStore, error) {
	if !opts.InMemory {
		dir := filepath.Dir(opts.Path)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("creating db directory: %w", err)
		}
	}

	dsn := opts.Path
	if opts.InMemory {
		dsn = ":memory:"
	}

	dsn += fmt.Sprintf("?_pragma=busy_timeout(%d)&_pragma=foreign_keys(1)", opts.BusyTimeout.Milliseconds())

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	db.SetMaxOpenConns(opts.MaxConns)
	db.SetMaxIdleConns(opts.MaxConns / 2)
	db.SetConnMaxLifetime(time.Hour)

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA mmap_size=268435456",
		"PRAGMA page_size=4096",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("setting pragma %s: %w", pragma, err)
		}
	}

	store := &SQLiteStore{
		db:     db,
		path:   opts.Path,
		logger: opts.Logger,
		stmts:  make(map[string]*sql.Stmt),
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		display_name TEXT NOT NULL,
		email TEXT,
		role TEXT NOT NULL DEFAULT 'user',
		zone TEXT NOT NULL DEFAULT 'default',
		password_hash TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen_at DATETIME,
		metadata JSON
	);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_zone ON users(zone);

	CREATE TABLE IF NOT EXISTS services (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		version TEXT NOT NULL,
		node_id TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		zone TEXT NOT NULL,
		status TEXT DEFAULT 'unknown',
		health_score REAL DEFAULT 1.0,
		registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_health_check DATETIME,
		metadata JSON,
		UNIQUE(name, node_id)
	);
	CREATE INDEX IF NOT EXISTS idx_services_name ON services(name);
	CREATE INDEX IF NOT EXISTS idx_services_zone ON services(zone);

	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'node',
		address TEXT NOT NULL,
		port INTEGER NOT NULL,
		zone TEXT NOT NULL,
		status TEXT DEFAULT 'unknown',
		last_seen DATETIME,
		registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata JSON
	);
	CREATE INDEX IF NOT EXISTS idx_nodes_zone ON nodes(zone);

	CREATE TABLE IF NOT EXISTS zones (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		description TEXT,
		subnet TEXT,
		campus TEXT,
		building TEXT,
		floor TEXT,
		access_level INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata JSON
	);

	CREATE TABLE IF NOT EXISTS attendance (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		zone_id TEXT NOT NULL,
		check_in DATETIME NOT NULL,
		check_out DATETIME,
		duration_minutes INTEGER,
		status TEXT DEFAULT 'present',
		verified_by TEXT,
		metadata JSON
	);
	CREATE INDEX IF NOT EXISTS idx_attendance_user ON attendance(user_id);
	CREATE INDEX IF NOT EXISTS idx_attendance_date ON attendance(check_in);

	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		event_type TEXT NOT NULL,
		user_id TEXT,
		service_id TEXT,
		zone_id TEXT,
		ip_address TEXT,
		details JSON,
		severity TEXT DEFAULT 'info'
	);
	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);

	CREATE TABLE IF NOT EXISTS plugins (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		version TEXT NOT NULL,
		description TEXT,
		author TEXT,
		enabled INTEGER DEFAULT 1,
		config JSON,
		installed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// DB returns the underlying *sql.DB
func (s *SQLiteStore) DB() *sql.DB {
	return s.db
}

// Exec executes a query without returning rows
func (s *SQLiteStore) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrStoreClosed
	}
	s.mu.RUnlock()
	return s.db.ExecContext(ctx, query, args...)
}

// QueryRow executes a query returning a single row
func (s *SQLiteStore) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

// Query executes a query returning multiple rows
func (s *SQLiteStore) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrStoreClosed
	}
	s.mu.RUnlock()
	return s.db.QueryContext(ctx, query, args...)
}

// Transaction executes a function within a transaction
func (s *SQLiteStore) Transaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	s.mu.RUnlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Checkpoint forces a WAL checkpoint
func (s *SQLiteStore) Checkpoint(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

// Stats returns database statistics
func (s *SQLiteStore) Stats() map[string]any {
	var pageCount, pageSize int64
	s.db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	s.db.QueryRow("PRAGMA page_size").Scan(&pageSize)

	var fileSize int64
	if fi, err := os.Stat(s.path); err == nil {
		fileSize = fi.Size()
	}

	return map[string]any{
		"path":       s.path,
		"file_size":  fileSize,
		"page_count": pageCount,
		"page_size":  pageSize,
		"db_size":    pageCount * pageSize,
	}
}

// Close gracefully closes the database
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	s.stmtMu.Lock()
	for _, stmt := range s.stmts {
		stmt.Close()
	}
	s.stmts = nil
	s.stmtMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Checkpoint(ctx)

	return s.db.Close()
}

// IsNotFound checks if error is "no rows"
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrKeyNotFound)
}
