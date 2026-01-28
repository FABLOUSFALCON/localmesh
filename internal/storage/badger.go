// Package storage provides fast, embedded database backends.
// Badger: Blazing fast key-value store for sessions, tokens, cache.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
)

var (
	// ErrKeyNotFound is returned when a key doesn't exist
	ErrKeyNotFound = errors.New("key not found")

	// ErrStoreClosed is returned when operating on closed store
	ErrStoreClosed = errors.New("store is closed")
)

// BadgerStore is a fast key-value store using Badger
type BadgerStore struct {
	db     *badger.DB
	path   string
	mu     sync.RWMutex
	closed bool
	logger *slog.Logger

	// Background goroutine control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// BadgerOptions configures the Badger store
type BadgerOptions struct {
	Path            string
	InMemory        bool          // For testing
	SyncWrites      bool          // Durability vs speed tradeoff
	GCInterval      time.Duration // Garbage collection interval
	GCDiscardRatio  float64       // GC discard ratio (0.5 = 50%)
	Logger          *slog.Logger
	CompactOnClose  bool
	NumCompactors   int
	NumMemtables    int
	ValueLogMaxSize int64
}

// DefaultBadgerOptions returns optimized defaults for LocalMesh
func DefaultBadgerOptions(path string) BadgerOptions {
	return BadgerOptions{
		Path:            path,
		InMemory:        false,
		SyncWrites:      false, // Async for speed, we have backups
		GCInterval:      5 * time.Minute,
		GCDiscardRatio:  0.5,
		CompactOnClose:  true,
		NumCompactors:   2,
		NumMemtables:    3,
		ValueLogMaxSize: 64 << 20, // 64MB value log files
	}
}

// NewBadgerStore creates a new Badger key-value store
func NewBadgerStore(opts BadgerOptions) (*BadgerStore, error) {
	// Configure Badger options for speed
	badgerOpts := badger.DefaultOptions(opts.Path)

	if opts.InMemory {
		badgerOpts = badgerOpts.WithInMemory(true)
	}

	// Performance tuning
	badgerOpts = badgerOpts.
		WithSyncWrites(opts.SyncWrites).
		WithNumCompactors(opts.NumCompactors).
		WithNumMemtables(opts.NumMemtables).
		WithValueLogFileSize(opts.ValueLogMaxSize).
		WithDetectConflicts(false) // We handle our own conflicts

	// Silence Badger's internal logging (use our slog)
	badgerOpts = badgerOpts.WithLogger(nil)

	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, fmt.Errorf("opening badger: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	store := &BadgerStore{
		db:     db,
		path:   opts.Path,
		logger: opts.Logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// Start garbage collection
	if opts.GCInterval > 0 && !opts.InMemory {
		store.wg.Add(1)
		go store.runGC(opts.GCInterval, opts.GCDiscardRatio)
	}

	return store, nil
}

// Set stores a value with optional TTL
func (s *BadgerStore) Set(key string, value []byte, ttl time.Duration) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	s.mu.RUnlock()

	return s.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), value)
		if ttl > 0 {
			entry = entry.WithTTL(ttl)
		}
		return txn.SetEntry(entry)
	})
}

// SetJSON stores a JSON-serializable value
func (s *BadgerStore) SetJSON(key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling value: %w", err)
	}
	return s.Set(key, data, ttl)
}

// Get retrieves a value by key
func (s *BadgerStore) Get(key string) ([]byte, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrStoreClosed
	}
	s.mu.RUnlock()

	var value []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return ErrKeyNotFound
			}
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})

	return value, err
}

// GetJSON retrieves and unmarshals a JSON value
func (s *BadgerStore) GetJSON(key string, dest any) error {
	data, err := s.Get(key)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete removes a key
func (s *BadgerStore) Delete(key string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	s.mu.RUnlock()

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// Exists checks if a key exists
func (s *BadgerStore) Exists(key string) (bool, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return false, ErrStoreClosed
	}
	s.mu.RUnlock()

	var exists bool
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key))
		if err == nil {
			exists = true
			return nil
		}
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil
		}
		return err
	})

	return exists, err
}

// Keys returns all keys with a given prefix
func (s *BadgerStore) Keys(prefix string) ([]string, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrStoreClosed
	}
	s.mu.RUnlock()

	var keys []string
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // Keys only
		opts.Prefix = []byte(prefix)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			keys = append(keys, string(it.Item().Key()))
		}
		return nil
	})

	return keys, err
}

// Scan iterates over keys with a prefix, calling fn for each
func (s *BadgerStore) Scan(prefix string, fn func(key string, value []byte) error) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	s.mu.RUnlock()

	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				return fn(string(item.Key()), val)
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// DeletePrefix removes all keys with a given prefix
func (s *BadgerStore) DeletePrefix(prefix string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	s.mu.RUnlock()

	return s.db.DropPrefix([]byte(prefix))
}

// TTL returns the remaining TTL for a key
func (s *BadgerStore) TTL(key string) (time.Duration, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return 0, ErrStoreClosed
	}
	s.mu.RUnlock()

	var ttl time.Duration
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return ErrKeyNotFound
			}
			return err
		}

		expiresAt := item.ExpiresAt()
		if expiresAt == 0 {
			ttl = 0 // No expiration
			return nil
		}

		ttl = time.Until(time.Unix(int64(expiresAt), 0))
		if ttl < 0 {
			ttl = 0
		}
		return nil
	})

	return ttl, err
}

// runGC runs periodic garbage collection
func (s *BadgerStore) runGC(interval time.Duration, discardRatio float64) {
	defer s.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			for {
				err := s.db.RunValueLogGC(discardRatio)
				if err != nil {
					break // No more GC needed
				}
			}
		}
	}
}

// Stats returns database statistics
func (s *BadgerStore) Stats() map[string]any {
	lsm, vlog := s.db.Size()
	return map[string]any{
		"lsm_size_bytes":   lsm,
		"vlog_size_bytes":  vlog,
		"total_size_bytes": lsm + vlog,
		"path":             s.path,
	}
}

// Backup creates a backup of the database
func (s *BadgerStore) Backup(path string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	s.mu.RUnlock()

	// Create backup file
	f, err := createBackupFile(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Stream backup
	_, err = s.db.Backup(f, 0)
	return err
}

// Restore restores from a backup
func (s *BadgerStore) Restore(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := openBackupFile(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return s.db.Load(f, 256)
}

// Compact forces compaction
func (s *BadgerStore) Compact() error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	s.mu.RUnlock()

	return s.db.Flatten(4) // 4 concurrent workers
}

// Close gracefully closes the store
func (s *BadgerStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	// Stop background goroutines
	s.cancel()
	s.wg.Wait()

	return s.db.Close()
}
