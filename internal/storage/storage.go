// Package storage provides unified access to all storage backends.
package storage

import (
	"fmt"
	"log/slog"
	"os"
)

// Storage provides unified access to all storage backends
type Storage struct {
	SQLite *SQLiteStore
	Badger *BadgerStore
	logger *slog.Logger
}

// Options configures the storage layer
type Options struct {
	SQLitePath string
	BadgerPath string
	InMemory   bool
	Logger     *slog.Logger
}

// New creates a new unified storage instance
func New(opts Options) (*Storage, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	sqliteOpts := DefaultSQLiteOptions(opts.SQLitePath)
	sqliteOpts.InMemory = opts.InMemory
	sqliteOpts.Logger = logger

	sqlite, err := NewSQLiteStore(sqliteOpts)
	if err != nil {
		return nil, fmt.Errorf("initializing sqlite: %w", err)
	}

	badgerOpts := DefaultBadgerOptions(opts.BadgerPath)
	badgerOpts.InMemory = opts.InMemory
	badgerOpts.Logger = logger

	badger, err := NewBadgerStore(badgerOpts)
	if err != nil {
		sqlite.Close()
		return nil, fmt.Errorf("initializing badger: %w", err)
	}

	return &Storage{
		SQLite: sqlite,
		Badger: badger,
		logger: logger,
	}, nil
}

// Close gracefully closes all storage backends
func (s *Storage) Close() error {
	var errs []error

	if s.SQLite != nil {
		if err := s.SQLite.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing sqlite: %w", err))
		}
	}

	if s.Badger != nil {
		if err := s.Badger.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing badger: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("storage close errors: %v", errs)
	}
	return nil
}

// Stats returns statistics from all storage backends
func (s *Storage) Stats() map[string]any {
	return map[string]any{
		"sqlite": s.SQLite.Stats(),
		"badger": s.Badger.Stats(),
	}
}

// Helper functions for backup file handling
func createBackupFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
}

func openBackupFile(path string) (*os.File, error) {
	return os.Open(path)
}
