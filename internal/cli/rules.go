package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"contextsync/internal/config"
	"contextsync/internal/rules"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage rules",
}

var rulesEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open rules file in editor",
	Run: func(cmd *cobra.Command, args []string) {
		rulesPath := config.GetRulesPath()
		fmt.Printf("Rules file: %s\n", rulesPath)
		fmt.Println("Open this file in your favorite editor.")
	},
}

var rulesShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current rules",
	Run: func(cmd *cobra.Command, args []string) {
		engine := rules.NewEngine()
		content, err := engine.GetRules("")
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}
		fmt.Println(content)
	},
}

var rulesSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync rules to all configured tools",
	Run: func(cmd *cobra.Command, args []string) {
		runRulesSync()
	},
}

func runRulesSync() {
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

	// Initialize database
	if err := initDatabase(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer closeDatabase()

	// Check license status
	isPro := false
	maxTools := 2
	if validator != nil {
		isPro = validator.IsPro()
		maxTools = validator.GetMaxTools()
	}

	// Get configured tools from database
	configuredTools := getConfiguredTools()

	if len(configuredTools) == 0 {
		fmt.Println(warnStyle.Render("  No tools configured. Run: contextsync init"))
		return
	}

	// Free tier: limit to maxTools
	if !isPro && len(configuredTools) > maxTools {
		fmt.Println(warnStyle.Render(fmt.Sprintf("  Free tier: syncing only first %d tools", maxTools)))
		configuredTools = configuredTools[:maxTools]
	}

	// Read rules content
	home, _ := os.UserHomeDir()
	rulesPath := filepath.Join(home, ".contextsync", "rules.md")
	content, err := os.ReadFile(rulesPath)
	if err != nil {
		fmt.Printf("Error reading rules: %v\n", err)
		return
	}

	// Tool name to target file mapping
	targetMapping := map[string]string{
		"Claude Code":    filepath.Join(home, ".claude", "CLAUDE.md"),
		"Cursor":         filepath.Join(home, ".cursorrules"),
		"Windsurf":       filepath.Join(home, ".codeium", "windsurfrules"),
		"Gemini CLI":     filepath.Join(home, ".gemini", "GEMINI.md"),
		"Codex CLI":      filepath.Join(home, ".codex", "AGENTS.md"),
		"GitHub Copilot": filepath.Join(home, ".github", "copilot-instructions.md"),
		"Cline":          filepath.Join(home, ".cline", "CLINE.md"),
		"Roo Code":       filepath.Join(home, ".roo", "ROO.md"),
		"Aider":          filepath.Join(home, ".aider", "AIDER.md"),
		"Continue":       filepath.Join(home, ".continue", "CONTINUE.md"),
		"Zed":            filepath.Join(home, ".zed", "ZED.md"),
		"Replit AI":      filepath.Join(home, ".replit", "REPLIT.md"),
	}

	fmt.Println(infoStyle.Render("  Syncing rules to configured tools..."))

	syncedCount := 0
	for _, toolName := range configuredTools {
		targetPath, ok := targetMapping[toolName]
		if !ok {
			continue
		}

		// Ensure directory exists
		dir := filepath.Dir(targetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("  %s: failed to create directory\n", toolName)
			continue
		}

		// Write file
		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			fmt.Printf("  %s: %v\n", toolName, err)
		} else {
			fmt.Printf("  %s: synced\n", toolName)
			syncedCount++
		}
	}

	fmt.Println()
	fmt.Println(successStyle.Render(fmt.Sprintf("  Rules synced to %d tool(s).", syncedCount)))
}

func getConfiguredTools() []string {
	if database == nil {
		return nil
	}

	rows, err := database.DB().Query("SELECT tool_name FROM configured_tools")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var tools []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tools = append(tools, name)
		}
	}
	return tools
}

func init() {
	rulesCmd.AddCommand(rulesEditCmd)
	rulesCmd.AddCommand(rulesShowCmd)
	rulesCmd.AddCommand(rulesSyncCmd)
}
