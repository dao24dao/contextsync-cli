package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"contextsync/internal/config"
	"contextsync/internal/daemon"
	"contextsync/internal/integrations"
	toolstore "contextsync/internal/tools"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize ContextSync and configure AI tools",
	Long: `Initialize ContextSync and automatically configure all detected AI coding tools.

This command will:
1. Create the ~/.contextsync directory structure
2. Initialize the SQLite database
3. Detect installed AI tools (12+ supported)
4. Configure MCP server for each tool (Free: max 2 tools)
5. Create default rules file`,
	Run: func(cmd *cobra.Command, args []string) {
		runInit()
	},
}

func runInit() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	proStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5CF6"))

	fmt.Println(titleStyle.Render("\nContextSync Initialization\n"))

	// Check if logged in
	if !config.IsLoggedIn() {
		fmt.Println(warnStyle.Render("  Not logged in"))
		fmt.Println("\n  Please login first to use ContextSync:")
		fmt.Println("    contextsync login\n")
		return
	}

	fmt.Printf("  Account: %s\n\n", config.GetAccountEmail())

	// Step 1: Create directory structure
	fmt.Println(infoStyle.Render("  Creating directory structure..."))
	if err := config.CreateDirectories(); err != nil {
		fmt.Printf("  Failed: %v\n", err)
		return
	}
	fmt.Println(successStyle.Render("  Created ~/.contextsync/\n"))

	// Step 2: Initialize database
	fmt.Println(infoStyle.Render("  Initializing database..."))
	if err := initDatabase(); err != nil {
		fmt.Printf("  Failed: %v\n", err)
		return
	}
	defer closeDatabase()
	fmt.Println(successStyle.Render("  Database initialized\n"))

	// Register device with server
	fmt.Println(infoStyle.Render("  Registering device..."))
	if err := registerDevice(); err != nil {
		fmt.Printf("  Error: %v\n", err)
		fmt.Println("\n  Device registration failed. Please check your network and try again.")
		return
	}
	fmt.Println(successStyle.Render("  Device registered\n"))

	// Check license status
	maxTools := validator.GetMaxTools()

	if validator.IsPro() {
		subType := validator.GetSubscriptionType()
		fmt.Printf("  %s", proStyle.Render("Pro License Active"))
		if subType != "" {
			fmt.Printf(" (%s)", subType)
		}
		fmt.Println("\n")
	} else {
		fmt.Printf("  Free tier: Max %d tools\n\n", maxTools)
	}

	// Step 3: Detect AI tools
	fmt.Println(infoStyle.Render("  Detecting AI tools..."))
	detector := integrations.NewDetector()
	detectedTools := detector.DetectAll()

	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	disabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))

	if len(detectedTools) == 0 {
		fmt.Println("  No AI tools detected")
	} else {
		for _, tool := range detectedTools {
			fmt.Printf("  Found: %s\n", tool.Name)
		}
	}
	fmt.Println()

	// Step 4: Tool selection for Free tier
	var toolsToConfigure []*integrations.Tool

	if !validator.IsPro() && len(detectedTools) > maxTools {
		toolsToConfigure = selectToolsInteractive(detectedTools, maxTools, titleStyle, selectedStyle, disabledStyle, hintStyle)
	} else {
		toolsToConfigure = detectedTools
	}

	// Step 5: Configure MCP for each tool
	if len(toolsToConfigure) > 0 {
		fmt.Println(infoStyle.Render("  Configuring MCP server..."))

		for _, tool := range toolsToConfigure {
			if err := tool.Configure(); err != nil {
				fmt.Printf("  %s: %v\n", tool.Name, err)
			} else {
				fmt.Printf("  Configured: %s\n", tool.Name)
				recordToolConfiguration(tool.Name, tool.ConfigPath)
			}
		}
		fmt.Println()

		// Save encrypted tool list to tools.enc
		toolNames := make([]string, len(toolsToConfigure))
		for i, t := range toolsToConfigure {
			toolNames[i] = t.Name
		}
		if err := toolstore.Save(&toolstore.ToolStore{
			Tools:     toolNames,
			AccountID: config.GetAccountID(),
		}); err != nil {
			fmt.Printf("  Warning: Failed to save tool config: %v\n", err)
		} else {
			fmt.Println(successStyle.Render("  Tool configuration saved"))
		}

		// Sync to cloud (best effort)
		if err := syncToolsToCloud(toolNames); err != nil {
			fmt.Printf("  Warning: Cloud sync skipped: %v\n", err)
		} else {
			fmt.Println(successStyle.Render("  Tool list synced to cloud"))
		}
		fmt.Println()
	}

	// Step 6: Create default rules
	fmt.Println(infoStyle.Render("  Creating default rules file..."))
	if err := config.CreateDefaultRules(); err != nil {
		fmt.Printf("  %v\n", err)
	} else {
		fmt.Println(successStyle.Render("  Created ~/.contextsync/rules.md\n"))
	}

	// Step 7: Install daemon service
	fmt.Println(infoStyle.Render("  Setting up daemon..."))
	svc := daemon.NewServiceManager()

	if svc.IsInstalled() {
		fmt.Println("  Daemon already installed")
	} else {
		if err := svc.Install(); err != nil {
			fmt.Printf("  Warning: Failed to install daemon: %v\n", err)
		} else {
			fmt.Println(successStyle.Render("  Daemon service installed"))
			if err := svc.Start(); err != nil {
				fmt.Printf("  Warning: Failed to start daemon: %v\n", err)
			} else {
				fmt.Println(successStyle.Render("  Daemon started"))
			}
		}
	}
	fmt.Println()

	// Done
	fmt.Println(titleStyle.Render("ContextSync initialized successfully! (2/2)\n"))
	fmt.Println("  Next steps:")
	fmt.Println("  1. Edit your rules: contextsync rules edit")
	fmt.Println("  2. View status: contextsync status")
	fmt.Println("  3. Start MCP server: contextsync server\n")
}

func getPreviouslyConfiguredCount() int {
	if database == nil {
		return 0
	}

	var count int
	database.DB().QueryRow("SELECT COUNT(*) FROM configured_tools").Scan(&count)
	return count
}

func recordToolConfiguration(name, configPath string) {
	if database == nil {
		return
	}

	database.DB().Exec(`
		INSERT OR REPLACE INTO configured_tools (tool_name, config_path, configured_at)
		VALUES (?, ?, datetime('now'))
	`, name, configPath)
}

func init() {
	initCmd.Flags().BoolP("force", "f", false, "Force configure all tools (ignores limits)")
}

// syncToolsToCloud syncs the tool list to the server (best effort)
func syncToolsToCloud(toolNames []string) error {
	serverURL := config.GetServerURL()
	token := config.GetAuthToken()
	accountID := config.GetAccountID()
	if serverURL == "" || token == "" || accountID == "" {
		return fmt.Errorf("not logged in")
	}

	body := map[string]interface{}{
		"tools": toolNames,
	}
	jsonBody, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		serverURL+"/api/v1/tools",
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

// selectToolsInteractive displays an interactive tool selector for Free tier
func selectToolsInteractive(tools []*integrations.Tool, maxSelect int, titleStyle, selectedStyle, disabledStyle, hintStyle lipgloss.Style) []*integrations.Tool {
	selected := make(map[int]bool)
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println()
		fmt.Println(titleStyle.Render("  Select " + fmt.Sprint(maxSelect) + " tools to configure:"))
		fmt.Println()

		for i, tool := range tools {
			prefix := "  [ ] "
			toolName := tool.Name
			style := lipgloss.NewStyle()

			if selected[i] {
				prefix = "  [x] "
				style = selectedStyle
			} else if len(selected) >= maxSelect {
				prefix = "  [ ] "
				toolName += " (disabled)"
				style = disabledStyle
			}

			fmt.Println(style.Render(prefix + toolName))
		}

		fmt.Println()
		fmt.Print(hintStyle.Render("  Enter numbers to toggle (e.g., 1,3) or press Enter to confirm: "))

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			if len(selected) == maxSelect {
				break
			}
			fmt.Println(hintStyle.Render("  Please select exactly " + fmt.Sprint(maxSelect) + " tools."))
			continue
		}

		parts := strings.Split(input, ",")
		for _, part := range parts {
			var num int
			if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &num); err == nil {
				idx := num - 1
				if idx >= 0 && idx < len(tools) {
					if selected[idx] {
						delete(selected, idx)
					} else if len(selected) < maxSelect {
						selected[idx] = true
					}
				}
			}
		}
	}

	var result []*integrations.Tool
	for i := range selected {
		result = append(result, tools[i])
	}
	return result
}
