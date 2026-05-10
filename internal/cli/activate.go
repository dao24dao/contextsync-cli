package cli

import (
	"fmt"

	"contextsync/internal/config"
	"contextsync/internal/license"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var activateCmd = &cobra.Command{
	Use:   "activate <license-key>",
	Short: "Activate Pro license",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		activateLicense(args[0])
	},
}

func activateLicense(key string) {
	titleStyle := lipgloss.NewStyle().Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	fmt.Println(titleStyle.Render("\n  Activating License...\n"))

	ensureDatabase()
	validator := license.NewValidator(config.GetServerURL())
	validator.SetDB(database)

	if err := validator.Activate(key); err != nil {
		fmt.Println(errorStyle.Render("  Activation failed"))
		fmt.Printf("  Error: %v\n\n", err)
		return
	}

	fmt.Println(successStyle.Render("  License activated!\n"))
	fmt.Println("  Pro features unlocked:")
	fmt.Println("    - All 6+ tools enabled")
	fmt.Println("    - Permanent memory retention")
	fmt.Println("    - Unlimited memory storage")
	fmt.Println("    - Cloud sync enabled\n")
}

var deactivateCmd = &cobra.Command{
	Use:   "deactivate",
	Short: "Deactivate current license",
	Run: func(cmd *cobra.Command, args []string) {
		deactivateLicense()
	},
}

func deactivateLicense() {
	ensureDatabase()
	validator := license.NewValidator(config.GetServerURL())
	validator.SetDB(database)
	validator.Deactivate()

	fmt.Println("\n  License deactivated.")
	fmt.Println("  You are now on the Free tier.\n")
}
