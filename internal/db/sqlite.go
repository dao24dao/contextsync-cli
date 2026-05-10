package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type SQLite struct {
	db *sql.DB
}

// NewSQLite creates a new SQLite connection
func NewSQLite(dbPath string) (*SQLite, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set pragmas for performance
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -64000", // 64MB
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &SQLite{db: db}, nil
}

// Close closes the database connection
func (s *SQLite) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection
func (s *SQLite) DB() *sql.DB {
	return s.db
}

// runMigrations runs database migrations
func runMigrations(db *sql.DB) error {
	// Create memories table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'other',
			source TEXT NOT NULL DEFAULT 'manual',
			project TEXT,
			tags TEXT DEFAULT '[]',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			device_id TEXT NOT NULL,
			synced INTEGER DEFAULT 0,
			raw_context TEXT,
			expires_at TEXT,
			dedup_hash TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_memories_category ON memories(category);
		CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at);
		CREATE INDEX IF NOT EXISTS idx_memories_dedup ON memories(dedup_hash);
		CREATE INDEX IF NOT EXISTS idx_memories_expires ON memories(expires_at);

		-- FTS5 for full-text search
		CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			content,
			content='memories',
			content_rowid='rowid'
		);

		-- Triggers to keep FTS in sync
		CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
			INSERT INTO memories_fts(rowid, content) VALUES (new.rowid, new.content);
		END;

		CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, content) VALUES('delete', old.rowid, old.content);
		END;

		CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, content) VALUES('delete', old.rowid, old.content);
			INSERT INTO memories_fts(rowid, content) VALUES (new.rowid, new.content);
		END;

		-- License table (tracks trial and subscription)
		CREATE TABLE IF NOT EXISTS license (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			tier TEXT NOT NULL DEFAULT 'free',
			license_key TEXT,
			subscription_id TEXT,
			subscription_type TEXT,
			valid_until TEXT,
			first_seen_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);

		INSERT OR IGNORE INTO license (id, tier, created_at, updated_at)
		VALUES (1, 'free', datetime('now'), datetime('now'));

		-- Configured tools table (for tool limit enforcement)
		CREATE TABLE IF NOT EXISTS configured_tools (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tool_name TEXT NOT NULL UNIQUE,
			config_path TEXT NOT NULL,
			configured_at TEXT NOT NULL
		);

		-- Config table for settings like last_sync
		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	// Run schema migrations for existing databases
	migrations := []string{
		// Add first_seen_at column if not exists
		`ALTER TABLE license ADD COLUMN first_seen_at TEXT`,
		// Add subscription_type column if not exists
		`ALTER TABLE license ADD COLUMN subscription_type TEXT`,
		// Add expires_at index if not exists
		`CREATE INDEX IF NOT EXISTS idx_memories_expires ON memories(expires_at)`,
	}

	for _, m := range migrations {
		// Ignore errors (column/index may already exist)
		db.Exec(m)
	}

	// Set first_seen_at for existing records
	db.Exec(`UPDATE license SET first_seen_at = datetime('now') WHERE first_seen_at IS NULL`)

	return nil
}
