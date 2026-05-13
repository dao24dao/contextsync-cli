package daemon

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"time"

	"contextsync/internal/cloud"
	"contextsync/internal/config"
	"contextsync/internal/memory"
	"contextsync/internal/rules"

	"github.com/fsnotify/fsnotify"
)

type Daemon struct {
	rulesPath   string
	db          *sql.DB
	cloudClient *cloud.Client
	validator   ProChecker

	stopChan chan struct{}
	wg       sync.WaitGroup

	// State tracking
	lastRulesMod  time.Time
	lastMemoryMod time.Time

	// Configuration
	syncInterval time.Duration
	debounceTime time.Duration
}

type ProChecker interface {
	IsPro() bool
}

type Option func(*Daemon)

func WithSyncInterval(d time.Duration) Option {
	return func(dmn *Daemon) {
		dmn.syncInterval = d
	}
}

func WithDebounceTime(d time.Duration) Option {
	return func(dmn *Daemon) {
		dmn.debounceTime = d
	}
}

func New(db *sql.DB, validator ProChecker, opts ...Option) *Daemon {
	home, _ := os.UserHomeDir()

	d := &Daemon{
		rulesPath:    filepath.Join(home, ".contextsync", "rules.md"),
		db:           db,
		validator:    validator,
		stopChan:     make(chan struct{}),
		syncInterval: 5 * time.Minute,
		debounceTime: 500 * time.Millisecond,
	}

	for _, opt := range opts {
		opt(d)
	}

	serverURL := config.GetServerURL()
	if serverURL != "" {
		d.cloudClient = cloud.NewClient(serverURL)
	}

	return d
}

// Run starts the daemon and blocks until Stop is called
func (d *Daemon) Run() error {
	logger := GetLogger()
	logger.Info("ContextSync daemon starting...")

	// Initialize rules modification time
	if info, err := os.Stat(d.rulesPath); err == nil {
		d.lastRulesMod = info.ModTime()
	}

	// Initialize memory modification time
	d.lastMemoryMod = d.getLastMemoryMod()

	// Start file watcher for rules
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error("Failed to create file watcher: %v", err)
		return err
	}
	defer watcher.Close()

	// Watch rules.md directory (watching dir is more reliable than file)
	rulesDir := filepath.Dir(d.rulesPath)
	if err := watcher.Add(rulesDir); err != nil {
		logger.Error("Failed to watch rules directory: %v", err)
		return err
	}
	logger.Info("Watching rules: %s", d.rulesPath)

	// Start periodic sync goroutine
	d.wg.Add(1)
	go d.periodicSync()

	// Start memory poller goroutine
	d.wg.Add(1)
	go d.memoryPoller()

	// Debounce timer for rules changes
	var debounceTimer *time.Timer
	var pendingRulesSync bool

	// Event loop
	for {
		select {
		case <-d.stopChan:
			logger.Info("Daemon stopping...")
			watcher.Close()
			d.wg.Wait()
			logger.Info("Daemon stopped")
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				continue
			}
			// Check if it's rules.md
			if filepath.Base(event.Name) == "rules.md" && (event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) {
				// Debounce: reset timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				pendingRulesSync = true
				debounceTimer = time.AfterFunc(d.debounceTime, func() {
					if pendingRulesSync {
						d.syncRules()
						pendingRulesSync = false
					}
				})
			}

		case err, ok := <-watcher.Errors:
			if ok {
				logger.Error("Watcher error: %v", err)
			}
		}
	}
}

// Stop stops the daemon
func (d *Daemon) Stop() {
	close(d.stopChan)
}

// syncRules compiles and syncs rules to all tools
func (d *Daemon) syncRules() {
	logger := GetLogger()
	logger.Info("Rules changed, syncing to tools...")

	engine := rules.NewEngine()
	if err := engine.Compile(); err != nil {
		logger.Error("Failed to sync rules: %v", err)
		return
	}

	logger.Info("Rules synced successfully")

	// Update last mod time
	if info, err := os.Stat(d.rulesPath); err == nil {
		d.lastRulesMod = info.ModTime()
	}
}

// memoryPoller checks for memory database changes
func (d *Daemon) memoryPoller() {
	defer d.wg.Done()

	logger := GetLogger()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			currentMod := d.getLastMemoryMod()
			if currentMod.After(d.lastMemoryMod) {
				logger.Info("Memory database changed")
				d.lastMemoryMod = currentMod
				// Trigger cloud sync for Pro users
				if d.validator.IsPro() {
					d.syncMemories()
				}
			}
		}
	}
}

// periodicSync performs periodic cloud sync for Pro users
func (d *Daemon) periodicSync() {
	defer d.wg.Done()

	if !d.validator.IsPro() {
		return
	}

	logger := GetLogger()
	ticker := time.NewTicker(d.syncInterval)
	defer ticker.Stop()

	logger.Info("Periodic sync started, interval: %v", d.syncInterval)

	// Initial sync after 30 seconds
	time.Sleep(30 * time.Second)
	d.syncMemories()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.syncMemories()
		}
	}
}

// syncMemories syncs memories to cloud
func (d *Daemon) syncMemories() {
	if d.cloudClient == nil {
		return
	}

	logger := GetLogger()
	logger.Debug("Starting memory sync...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get license key
	var licenseKey string
	d.db.QueryRow("SELECT license_key FROM license WHERE id = 1").Scan(&licenseKey)
	if licenseKey == "" {
		logger.Debug("No license key, skipping sync")
		return
	}

	// Get device ID
	deviceID := config.GetDeviceID()

	// Get last sync time
	var lastSyncStr sql.NullString
	d.db.QueryRow("SELECT value FROM config WHERE key = 'last_sync'").Scan(&lastSyncStr)
	var lastSync int64
	if lastSyncStr.Valid && lastSyncStr.String != "" {
		t, _ := time.Parse(time.RFC3339, lastSyncStr.String)
		lastSync = t.Unix()
	}

	// Get unsynced memories
	rows, err := d.db.Query(`
		SELECT id, content, category, source, project, tags, created_at, updated_at, device_id
		FROM memories WHERE synced = 0
		LIMIT 100
	`)
	if err != nil {
		logger.Error("Failed to get unsynced memories: %v", err)
		return
	}

	var memories []*memory.Memory
	for rows.Next() {
		m := &memory.Memory{}
		var tagsJSON string
		if err := rows.Scan(&m.ID, &m.Content, &m.Category, &m.Source, &m.Project,
			&tagsJSON, &m.CreatedAt, &m.UpdatedAt, &m.DeviceID); err != nil {
			continue
		}
		memories = append(memories, m)
	}
	rows.Close()

	if len(memories) == 0 && lastSync == 0 {
		logger.Debug("No memories to sync")
		return
	}

	// Perform sync
	remoteMemories, deletedIDs, err := d.cloudClient.MergeAndSync(ctx, licenseKey, deviceID, memories, lastSync)
	if err != nil {
		logger.Error("Sync failed: %v", err)
		return
	}

	// Mark local as synced
	if len(memories) > 0 {
		for _, m := range memories {
			d.db.Exec("UPDATE memories SET synced = 1 WHERE id = ?", m.ID)
		}
	}

	// Upsert remote memories
	for _, rm := range remoteMemories {
		d.db.Exec(`
			INSERT OR REPLACE INTO memories (id, content, category, source, project, tags, created_at, updated_at, device_id, synced)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		`, rm.ID, rm.Content, rm.Category, rm.Source, rm.Project, "[]", rm.CreatedAt, rm.UpdatedAt, rm.DeviceID)
	}

	// Delete remotely deleted
	for _, id := range deletedIDs {
		d.db.Exec("DELETE FROM memories WHERE id = ?", id)
	}

	// Update last sync
	now := time.Now().Format(time.RFC3339)
	d.db.Exec(`INSERT OR REPLACE INTO config (key, value) VALUES ('last_sync', ?)`, now)

	logger.Info("Synced %d memories, received %d, deleted %d", len(memories), len(remoteMemories), len(deletedIDs))
}

// getLastMemoryMod returns the latest memory modification time
func (d *Daemon) getLastMemoryMod() time.Time {
	var lastMod time.Time
	d.db.QueryRow(`
		SELECT COALESCE(MAX(updated_at), '1970-01-01T00:00:00Z')
		FROM memories
	`).Scan(&lastMod)
	return lastMod
}

// RunOnce runs daemon logic once (for testing)
func RunOnce(db *sql.DB, validator ProChecker) {
	d := New(db, validator)
	d.syncRules()
	if validator.IsPro() {
		d.syncMemories()
	}
}
