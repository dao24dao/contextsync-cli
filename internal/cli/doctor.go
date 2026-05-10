package cli

import (
	"fmt"
	"os"
	"runtime"

	"contextsync/internal/config"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostics",
	Run: func(cmd *cobra.Command, args []string) {
		runDoctor()
	},
}

func runDoctor() {
	titleStyle := lipgloss.NewStyle().Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))

	fmt.Println(titleStyle.Render("\n  ContextSync Diagnostics\n"))

	// Check Go version
	fmt.Printf("  %-20s %s\n", "Go Version:", runtime.Version())
	fmt.Printf("  %-20s %s/%s\n", "Platform:", runtime.GOOS, runtime.GOARCH)

	// Check config directory
	configDir := config.GetConfigDir()
	if _, err := os.Stat(configDir); err == nil {
		fmt.Printf("  %-20s %s %s\n", "Config Dir:", configDir, successStyle.Render("✓"))
	} else {
		fmt.Printf("  %-20s %s %s\n", "Config Dir:", configDir, warnStyle.Render("(not found)"))
	}

	// Check database
	dbPath := config.GetDataPath()
	if _, err := os.Stat(dbPath); err == nil {
		fmt.Printf("  %-20s %s %s\n", "Database:", dbPath, successStyle.Render("✓"))
	} else {
		fmt.Printf("  %-20s %s %s\n", "Database:", dbPath, warnStyle.Render("(not found)"))
	}

	// Check rules file
	rulesPath := config.GetRulesPath()
	if _, err := os.Stat(rulesPath); err == nil {
		fmt.Printf("  %-20s %s %s\n", "Rules File:", rulesPath, successStyle.Render("✓"))
	} else {
		fmt.Printf("  %-20s %s %s\n", "Rules File:", rulesPath, warnStyle.Render("(not found)"))
	}

	// Check AI tools
	fmt.Println()
	fmt.Println(titleStyle.Render("  AI Tools:"))

	home, _ := os.UserHomeDir()
	tools := []struct {
		name string
		path string
	}{
		{"Claude Code", home + "/.claude"},
		{"Cursor", home + "/.cursor"},
		{"Gemini CLI", home + "/.gemini"},
		{"Windsurf", home + "/.codeium"},
		{"Codex CLI", home + "/.codex"},
	}

	for _, tool := range tools {
		if _, err := os.Stat(tool.path); err == nil {
			fmt.Printf("  %-20s %s\n", tool.name+":", successStyle.Render("✓ installed"))
		} else {
			fmt.Printf("  %-20s %s\n", tool.name+":", errorStyle.Render("✗ not found"))
		}
	}

	fmt.Println()
	fmt.Println("  Run 'contextsync init' to set up ContextSync.\n")
}
