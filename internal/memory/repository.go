package memory

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"contextsync/internal/db"
	"github.com/google/uuid"
)

type Category string

const (
	CategoryDecision     Category = "decision"
	CategoryPreference   Category = "preference"
	CategoryTodo         Category = "todo"
	CategoryErrorFix     Category = "error_fix"
	CategoryArchitecture Category = "architecture"
	CategoryOther        Category = "other"
)

type Memory struct {
	ID        string     `json:"id"`
	Content   string     `json:"content"`
	Category  Category   `json:"category"`
	Source    string     `json:"source"`
	Project   string     `json:"project,omitempty"`
	Tags      []string   `json:"tags"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeviceID  string     `json:"device_id"`
	Synced    bool       `json:"synced"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type Repository struct {
	db       *sql.DB
	isPro    func() bool
}

type RepositoryOption func(*Repository)

func WithProChecker(isPro func() bool) RepositoryOption {
	return func(r *Repository) {
		r.isPro = isPro
	}
}

func NewRepository(sqlite *db.SQLite, opts ...RepositoryOption) *Repository {
	r := &Repository{
		db: sqlite.DB(),
		isPro: func() bool { return false }, // Default: not Pro
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// SetProChecker sets the function to check Pro status
func (r *Repository) SetProChecker(isPro func() bool) {
	r.isPro = isPro
}

// Create creates a new memory
func (r *Repository) Create(content string, category string) (*Memory, error) {
	now := time.Now()
	id := uuid.New().String()
	deviceID := "local" // TODO: get from config

	// Calculate dedup hash
	hash := sha256.Sum256([]byte(category + ":" + content))
	dedupHash := hex.EncodeToString(hash[:])

	// Check for duplicates
	var existingID string
	err := r.db.QueryRow(
		"SELECT id FROM memories WHERE dedup_hash = ?",
		dedupHash,
	).Scan(&existingID)
	if err == nil {
		// Duplicate found, return existing
		return r.GetByID(existingID)
	}

	// Calculate expiry: Pro users get permanent storage, Free users get 14 days
	var expiresAt *time.Time
	if !r.isPro() {
		exp := now.AddDate(0, 0, 14)
		expiresAt = &exp
	}
	// If Pro, expiresAt remains nil (permanent)

	mem := &Memory{
		ID:        id,
		Content:   content,
		Category:  Category(category),
		Source:    "manual",
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
		DeviceID:  deviceID,
		Synced:    false,
		ExpiresAt: expiresAt,
	}

	tagsJSON, _ := json.Marshal(mem.Tags)

	var expiresAtStr interface{}
	if expiresAt != nil {
		expiresAtStr = expiresAt.Format(time.RFC3339)
	}

	_, err = r.db.Exec(`
		INSERT INTO memories (id, content, category, source, project, tags, created_at, updated_at, device_id, synced, expires_at, dedup_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, mem.ID, mem.Content, mem.Category, mem.Source, mem.Project, string(tagsJSON),
		mem.CreatedAt.Format(time.RFC3339), mem.UpdatedAt.Format(time.RFC3339),
		mem.DeviceID, mem.Synced, expiresAtStr, dedupHash)

	if err != nil {
		return nil, fmt.Errorf("failed to create memory: %w", err)
	}

	return mem, nil
}

// GetByID retrieves a memory by ID
func (r *Repository) GetByID(id string) (*Memory, error) {
	mem := &Memory{}
	var tagsJSON string
	var expiresAt sql.NullString

	err := r.db.QueryRow(`
		SELECT id, content, category, source, project, tags, created_at, updated_at, device_id, synced, expires_at
		FROM memories WHERE id = ?
	`, id).Scan(&mem.ID, &mem.Content, &mem.Category, &mem.Source, &mem.Project, &tagsJSON,
		&mem.CreatedAt, &mem.UpdatedAt, &mem.DeviceID, &mem.Synced, &expiresAt)

	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(tagsJSON), &mem.Tags)
	if expiresAt.Valid {
		t, _ := time.Parse(time.RFC3339, expiresAt.String)
		mem.ExpiresAt = &t
	}

	return mem, nil
}

// Search performs full-text search, excluding expired memories for Free users
func (r *Repository) Search(query string, limit int) []*Memory {
	var rows *sql.Rows
	var err error

	// For Free users, filter out expired memories
	if !r.isPro() {
		rows, err = r.db.Query(`
			SELECT m.id, m.content, m.category, m.source, m.project, m.tags, m.created_at, m.updated_at
			FROM memories m
			JOIN memories_fts fts ON m.rowid = fts.rowid
			WHERE memories_fts MATCH ?
			  AND (m.expires_at IS NULL OR m.expires_at > datetime('now'))
			ORDER BY m.created_at DESC
			LIMIT ?
		`, query, limit)
	} else {
		// Pro users see all memories
		rows, err = r.db.Query(`
			SELECT m.id, m.content, m.category, m.source, m.project, m.tags, m.created_at, m.updated_at
			FROM memories m
			JOIN memories_fts fts ON m.rowid = fts.rowid
			WHERE memories_fts MATCH ?
			ORDER BY m.created_at DESC
			LIMIT ?
		`, query, limit)
	}

	if err != nil {
		return nil
	}
	defer rows.Close()

	return r.scanMemories(rows)
}

// List returns memories with optional filter, excluding expired for Free users
func (r *Repository) List(category string, limit int) []*Memory {
	var rows *sql.Rows
	var err error

	// For Free users, filter out expired memories
	if !r.isPro() {
		if category != "" {
			rows, err = r.db.Query(`
				SELECT id, content, category, source, project, tags, created_at, updated_at
				FROM memories
				WHERE category = ?
				  AND (expires_at IS NULL OR expires_at > datetime('now'))
				ORDER BY created_at DESC
				LIMIT ?
			`, category, limit)
		} else {
			rows, err = r.db.Query(`
				SELECT id, content, category, source, project, tags, created_at, updated_at
				FROM memories
				WHERE expires_at IS NULL OR expires_at > datetime('now')
				ORDER BY created_at DESC
				LIMIT ?
			`, limit)
		}
	} else {
		// Pro users see all memories
		if category != "" {
			rows, err = r.db.Query(`
				SELECT id, content, category, source, project, tags, created_at, updated_at
				FROM memories
				WHERE category = ?
				ORDER BY created_at DESC
				LIMIT ?
			`, category, limit)
		} else {
			rows, err = r.db.Query(`
				SELECT id, content, category, source, project, tags, created_at, updated_at
				FROM memories
				ORDER BY created_at DESC
				LIMIT ?
			`, limit)
		}
	}

	if err != nil {
		return nil
	}
	defer rows.Close()

	return r.scanMemories(rows)
}

// Delete removes a memory
func (r *Repository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM memories WHERE id = ?", id)
	return err
}

// CleanExpired removes all expired memories (called periodically)
func (r *Repository) CleanExpired() (int64, error) {
	result, err := r.db.Exec(`
		DELETE FROM memories
		WHERE expires_at IS NOT NULL
		  AND expires_at < datetime('now')
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetUnsynced returns memories that haven't been synced to cloud
func (r *Repository) GetUnsynced(limit int) []*Memory {
	rows, err := r.db.Query(`
		SELECT id, content, category, source, project, tags, created_at, updated_at, device_id, synced
		FROM memories
		WHERE synced = 0
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		mem := &Memory{}
		var tagsJSON string
		var syncedInt int

		if err := rows.Scan(&mem.ID, &mem.Content, &mem.Category, &mem.Source, &mem.Project,
			&tagsJSON, &mem.CreatedAt, &mem.UpdatedAt, &mem.DeviceID, &syncedInt); err != nil {
			continue
		}

		json.Unmarshal([]byte(tagsJSON), &mem.Tags)
		mem.Synced = syncedInt == 1
		memories = append(memories, mem)
	}

	return memories
}

// MarkSynced marks memories as synced
func (r *Repository) MarkSynced(ids []string) error {
	for _, id := range ids {
		_, err := r.db.Exec("UPDATE memories SET synced = 1 WHERE id = ?", id)
		if err != nil {
			return err
		}
	}
	return nil
}

// Upsert inserts or updates a memory from cloud sync
func (r *Repository) Upsert(mem *Memory) error {
	tagsJSON, _ := json.Marshal(mem.Tags)

	var expiresAtStr interface{}
	if mem.ExpiresAt != nil {
		expiresAtStr = mem.ExpiresAt.Format(time.RFC3339)
	}

	synced := 0
	if mem.Synced {
		synced = 1
	}

	_, err := r.db.Exec(`
		INSERT INTO memories (id, content, category, source, project, tags, created_at, updated_at, device_id, synced, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content = excluded.content,
			category = excluded.category,
			updated_at = excluded.updated_at,
			synced = excluded.synced
	`, mem.ID, mem.Content, mem.Category, mem.Source, mem.Project, string(tagsJSON),
		mem.CreatedAt.Format(time.RFC3339), mem.UpdatedAt.Format(time.RFC3339),
		mem.DeviceID, synced, expiresAtStr)

	return err
}

// Stats holds memory statistics
type Stats struct {
	Total    int
	Expiring int
	Expired  int
}

// GetStats returns memory statistics
func (r *Repository) GetStats() Stats {
	var stats Stats

	// For Free users, only count non-expired
	if !r.isPro() {
		r.db.QueryRow(`
			SELECT COUNT(*) FROM memories
			WHERE expires_at IS NULL OR expires_at > datetime('now')
		`).Scan(&stats.Total)
	} else {
		r.db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&stats.Total)
	}

	r.db.QueryRow(`
		SELECT COUNT(*) FROM memories
		WHERE expires_at IS NOT NULL
		  AND expires_at > datetime('now')
		  AND expires_at < datetime('now', '+3 days')
	`).Scan(&stats.Expiring)

	r.db.QueryRow(`
		SELECT COUNT(*) FROM memories
		WHERE expires_at IS NOT NULL AND expires_at < datetime('now')
	`).Scan(&stats.Expired)

	return stats
}

func (r *Repository) scanMemories(rows *sql.Rows) []*Memory {
	var memories []*Memory

	for rows.Next() {
		mem := &Memory{}
		var tagsJSON string

		if err := rows.Scan(&mem.ID, &mem.Content, &mem.Category, &mem.Source, &mem.Project,
			&tagsJSON, &mem.CreatedAt, &mem.UpdatedAt); err != nil {
			continue
		}

		json.Unmarshal([]byte(tagsJSON), &mem.Tags)
		memories = append(memories, mem)
	}

	return memories
}
