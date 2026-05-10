package cli

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"contextsync/internal/cloud"
	"contextsync/internal/config"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync memories to cloud (Pro feature)",
	Long: `Sync your memories across all your devices.

This requires a Pro subscription. Free tier users can upgrade
to enable cloud sync.`,
	Run: func(cmd *cobra.Command, args []string) {
		runSync()
	},
}

func runSync() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))

	fmt.Println(titleStyle.Render("\n  ContextSync Cloud Sync\n"))

	// Check Pro status
	ensureDatabase()
	if !validator.IsPro() {
		prompt := validator.ShouldPromptForSync()
		fmt.Println(warnStyle.Render("  " + prompt.Trigger))
		fmt.Println()
		fmt.Println("  " + prompt.Message)
		fmt.Println("\n  Run: " + lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Render(prompt.Cta) + "\n")
		return
	}

	// Get license key
	var licenseKey string
	database.DB().QueryRow("SELECT license_key FROM license WHERE id = 1").Scan(&licenseKey)
	if licenseKey == "" {
		fmt.Println(errorStyle.Render("  No license key found. Please activate first.\n"))
		return
	}

	// Get device ID
	deviceID := config.GetDeviceID()

	// Get last sync time
	var lastSyncStr sql.NullString
	database.DB().QueryRow("SELECT value FROM config WHERE key = 'last_sync'").Scan(&lastSyncStr)
	var lastSync int64
	if lastSyncStr.Valid && lastSyncStr.String != "" {
		t, _ := time.Parse(time.RFC3339, lastSyncStr.String)
		lastSync = t.Unix()
	}

	// Get memory repository
	repo := getMemoryRepo()

	// Get all local memories count
	stats := repo.GetStats()

	// Get unsynced memories
	unsynced := repo.GetUnsynced(1000)

	fmt.Printf("  Local memories: %d total\n", stats.Total)
	fmt.Printf("  Pending sync:   %d\n\n", len(unsynced))

	// Create sync client
	serverURL := config.GetServerURL()
	if serverURL == "" {
		serverURL = "https://api.contextsync.dev"
	}
	client := cloud.NewClient(serverURL)

	// Perform sync
	fmt.Println("  Syncing with cloud...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	remoteMemories, deletedIDs, err := client.MergeAndSync(ctx, licenseKey, deviceID, unsynced, lastSync)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("\n  Sync failed: %v\n", err)))
		return
	}

	// Merge remote memories into local database
	syncedCount := 0
	for _, rm := range remoteMemories {
		if err := repo.Upsert(rm); err == nil {
			syncedCount++
		}
	}

	// Mark local memories as synced
	if len(unsynced) > 0 {
		ids := make([]string, len(unsynced))
		for i, m := range unsynced {
			ids[i] = m.ID
		}
		repo.MarkSynced(ids)
		syncedCount += len(unsynced)
	}

	// Delete remotely deleted memories
	for _, id := range deletedIDs {
		repo.Delete(id)
	}

	// Update last sync time
	now := time.Now().Format(time.RFC3339)
	database.DB().Exec(`
		INSERT OR REPLACE INTO config (key, value) VALUES ('last_sync', ?)
	`, now)

	fmt.Println(successStyle.Render("\n  Sync complete!\n"))
	fmt.Printf("  Synced:  %d memories\n", syncedCount)
	fmt.Printf("  Removed: %d memories\n", len(deletedIDs))
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
