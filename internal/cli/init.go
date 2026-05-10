package cli

import (
	"fmt"

	"contextsync/internal/config"
	"contextsync/internal/integrations"
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
		fmt.Printf("  Warning: %v\n", err)
	} else {
		fmt.Println(successStyle.Render("  Device registered\n"))
	}

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
	tools := detector.DetectAll()
	totalSupported := integrations.GetToolCount()

	if len(tools) == 0 {
		fmt.Println("  No AI tools detected")
	} else {
		for _, tool := range tools {
			fmt.Printf("  Found: %s\n", tool.Name)
		}
	}
	fmt.Println()

	// Step 4: Configure MCP for each tool (with limit enforcement)
	if len(tools) > 0 {
		fmt.Println(infoStyle.Render("  Configuring MCP server..."))

		// Check how many tools are already configured
		configuredCount := getPreviouslyConfiguredCount()
		newlyConfigured := 0
		skippedTools := 0

		for _, tool := range tools {
			// Check if we can configure more tools
			if !validator.IsPro() && (configuredCount+newlyConfigured) >= maxTools {
				fmt.Printf("  %s: skipped (Free tier limited to %d tools)\n", tool.Name, maxTools)
				skippedTools++
				continue
			}

			if err := tool.Configure(); err != nil {
				fmt.Printf("  %s: %v\n", tool.Name, err)
			} else {
				fmt.Printf("  Configured: %s\n", tool.Name)
				newlyConfigured++
				// Record in database
				recordToolConfiguration(tool.Name, tool.ConfigPath)
			}
		}

		// Show upgrade prompt if tools were skipped
		if skippedTools > 0 {
			fmt.Println()
			fmt.Printf("  %s\n", warnStyle.Render(fmt.Sprintf("Skipped %d tools. Pro supports all %d tools.", skippedTools, totalSupported)))
			fmt.Printf("  Upgrade: %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Render("contextsync upgrade"))
		}
		fmt.Println()
	}

	// Step 5: Create default rules
	fmt.Println(infoStyle.Render("  Creating default rules file..."))
	if err := config.CreateDefaultRules(); err != nil {
		fmt.Printf("  %v\n", err)
	} else {
		fmt.Println(successStyle.Render("  Created ~/.contextsync/rules.md\n"))
	}

	// Done
	fmt.Println(titleStyle.Render("ContextSync initialized successfully!\n"))
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
	// Add force flag to bypass limits
	initCmd.Flags().BoolP("force", "f", false, "Force configure all tools (ignores limits)")
}
