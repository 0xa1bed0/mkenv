package state

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type KVStoreKey string

// Entry represents one cache record: key -> value.
type Entry struct {
	Key       KVStoreKey
	Value     string
	CreatedAt time.Time
	LastUsed  time.Time
}

type KVStore struct {
	db *DB
}

// NewKVStore creates the store and ensures the table exists.
func NewKVStore(ctx context.Context, database *DB) (*KVStore, error) {
	if database == nil {
		return nil, nil
	}
	s := &KVStore{db: database}
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

var defaultKVStore *KVStore

func DefaultKVStore(ctx context.Context) (*KVStore, error) {
	if defaultKVStore == nil {
		db, err := OpenDefault(ctx)
		if err != nil {
			return nil, err
		}
		defaultKVStore, err = NewKVStore(ctx, db)
		if err != nil {
			return nil, err
		}
	}

	return defaultKVStore, nil
}

func (s *KVStore) ensureSchema(ctx context.Context) error {
	const createTable = `
CREATE TABLE IF NOT EXISTS kv_store (
	key        TEXT PRIMARY KEY,
	value      TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	last_used  INTEGER NOT NULL
);
`
	_, err := s.db.Raw().ExecContext(ctx, createTable)
	if err != nil {
		return fmt.Errorf("kvstore: ensure schema: %w", err)
	}
	return nil
}

// Get returns the cache entry for the given key.
// found == false means "no row".
func (s *KVStore) Get(ctx context.Context, key KVStoreKey) (entry Entry, found bool, err error) {
	const q = `
SELECT key, value, created_at, last_used
FROM kv_store
WHERE key = ?
`
	row := s.db.Raw().QueryRowContext(ctx, q, key)

	var createdAtUnix, lastUsedUnix int64
	if err = row.Scan(&entry.Key, &entry.Value, &createdAtUnix, &lastUsedUnix); err != nil {
		if err == sql.ErrNoRows {
			return Entry{}, false, nil
		}
		return Entry{}, false, fmt.Errorf("kv_Store: get: %w", err)
	}

	entry.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
	entry.LastUsed = time.Unix(lastUsedUnix, 0).UTC()

	// TODO: make configurable
	_ = s.Touch(ctx, key)

	return entry, true, nil
}

// Upsert sets value for the key. If the row exists,
// it updates the value + last_used; otherwise it inserts a new one.
func (s *KVStore) Upsert(ctx context.Context, key KVStoreKey, value string) error {
	const stmt = `
INSERT INTO kv_store (key, value, created_at, last_used)
VALUES (?, ?, strftime('%s','now'), strftime('%s','now'))
ON CONFLICT(key) DO UPDATE SET
  value = excluded.value,
	key = excluded.key,
	last_used = strftime('%s','now');
`

	if _, err := s.db.Raw().ExecContext(ctx, stmt, key, value); err != nil {
		return fmt.Errorf("kv_store: upsert: %w", err)
	}
	return nil
}

// Touch updates last_used for a given key if it exists.
// No-op if the row doesn't exist.
func (s *KVStore) Touch(ctx context.Context, key KVStoreKey) error {
	const stmt = `
UPDATE kv_store
SET last_used = strftime('%s','now')
WHERE key = ?;
`
	if _, err := s.db.Raw().ExecContext(ctx, stmt, key); err != nil {
		return fmt.Errorf("kv_store: touch: %w", err)
	}
	return nil
}

// Delete removes the entry for the given key, if any.
func (s *KVStore) Delete(ctx context.Context, key KVStoreKey) error {
	const stmt = `DELETE FROM kv_store WHERE key = ?`
	if _, err := s.db.Raw().ExecContext(ctx, stmt, key); err != nil {
		return fmt.Errorf("kv_store: delete: %w", err)
	}
	return nil
}

// DeleteUnusedBefore deletes entries that haven't been used since cutoff.
func (s *KVStore) DeleteUnusedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	const stmt = `
DELETE FROM kv_store
WHERE last_used < ?;
`
	res, err := s.db.Raw().ExecContext(ctx, stmt, cutoff.Unix())
	if err != nil {
		return 0, fmt.Errorf("kv_store: delete unused: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
