package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"contextsync/internal/config"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var rulesCloudSyncCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Sync rules.md to cloud (Pro feature)",
	Long: `Upload your local rules.md to the cloud for cross-device sync.

Requires Solo Pro subscription. Free tier users can upgrade to enable rules cloud sync.`,
	Run: func(cmd *cobra.Command, args []string) {
		runRulesCloudSync()
	},
}

func runRulesCloudSync() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	fmt.Println(titleStyle.Render("\n  ContextSync Rules Cloud Sync\n"))

	ensureDatabase()

	// Check Pro status
	if !validator.IsPro() {
		fmt.Println(warnStyle.Render("  Rules cloud sync requires ContextSync Pro."))
		fmt.Println()
		fmt.Println("  Free tier: local rules sync only")
		fmt.Println("  Pro tier: cross-device cloud sync for your rules.md")
		fmt.Println()
		fmt.Println("  Run: " + lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Render("contextsync upgrade"))
		fmt.Println()
		return
	}

	// Check logged in
	if !config.IsLoggedIn() {
		fmt.Println(errorStyle.Render("  Not logged in. Please login first."))
		fmt.Println()
		fmt.Println("  Run: contextsync login")
		fmt.Println()
		return
	}

	// Read local rules.md
	content, err := getRulesContent()
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("  Failed to read rules.md: %v", err)))
		return
	}

	if content == "" || len(content) < 10 {
		fmt.Println(warnStyle.Render("  Rules file is empty or very short. Create rules first:"))
		fmt.Println("  Run: contextsync rules edit")
		return
	}

	// Upload to cloud
	serverURL := config.GetServerURL()
	token := config.GetAuthToken()
	deviceID := config.GetDeviceID()

	body := map[string]string{
		"content":   content,
		"device_id": deviceID,
	}
	jsonBody, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		serverURL+"/api/v1/rules/sync",
		bytes.NewReader(jsonBody))
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("  Request error: %v", err)))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("  Network error: %v", err)))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		fmt.Println(errorStyle.Render(fmt.Sprintf("  Sync failed (HTTP %d): %s", resp.StatusCode, result.Error)))
		return
	}

	fmt.Println(successStyle.Render("  Rules synced to cloud!\n"))
	fmt.Println("  Your rules.md is now available on all your registered devices.")
	fmt.Println()
}

func syncRulesToCloud(content string) error {
	serverURL := config.GetServerURL()
	token := config.GetAuthToken()
	deviceID := config.GetDeviceID()
	if serverURL == "" || token == "" || deviceID == "" {
		return fmt.Errorf("not logged in")
	}

	body := map[string]string{
		"content":   content,
		"device_id": deviceID,
	}
	jsonBody, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		serverURL+"/api/v1/rules/sync",
		bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func init() {
	rulesCmd.AddCommand(rulesCloudSyncCmd)
}

// getRulesContent reads and returns the raw rules file content (moved from rules engine to avoid import).
func getRulesContent() (string, error) {
	rulesPath := config.GetRulesPath()
	content, err := os.ReadFile(rulesPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// getSyncRulesContent is the same as getRulesContent but for the init auto-sync function.
var getSyncRulesContent = func() (string, error) {
	return getRulesContent()
}
