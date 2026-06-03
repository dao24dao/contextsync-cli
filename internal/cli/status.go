package cli

import (
	"fmt"
	"time"

	"contextsync/internal/config"
	"contextsync/internal/license"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current status and configuration",
	Run: func(cmd *cobra.Command, args []string) {
		runStatus()
	},
}

func runStatus() {
	titleStyle := lipgloss.NewStyle().Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
	proStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	freeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

	fmt.Println(titleStyle.Render("\nContextSync Status\n"))

	// Version
	fmt.Printf("  %-12s %s\n", labelStyle.Render("Version:"), valueStyle.Render(version))

	// Device ID (always available locally)
	fmt.Printf("  %-12s %s\n", labelStyle.Render("Device ID:"), valueStyle.Render(config.GetDeviceID()))

	// Memory stats (always available locally)
	ensureDatabase()
	memRepo := getMemoryRepo()
	stats := memRepo.GetStats()

	fmt.Println()
	fmt.Println(titleStyle.Render("Local Storage:"))
	fmt.Printf("  %-12s %d memories\n", labelStyle.Render("Total:"), stats.Total)
	if stats.Expiring > 0 {
		fmt.Printf("  %-12s %d expiring soon\n", labelStyle.Render("Warning:"), stats.Expiring)
	}

	// Check login status
	if !config.IsLoggedIn() {
		// Not logged in - show limited info with friendly prompt
		fmt.Println()
		fmt.Println(titleStyle.Render("Account:"))
		fmt.Println(warnStyle.Render("  Not logged in"))

		// Configured tools (from local DB)
		var toolCount int
		database.DB().QueryRow("SELECT COUNT(*) FROM configured_tools").Scan(&toolCount)
		fmt.Println()
		fmt.Println(titleStyle.Render("Tools:"))
		fmt.Printf("  %-12s %d / 2\n", labelStyle.Render("Configured:"), toolCount)

		// Features available without login
		fmt.Println()
		fmt.Println(titleStyle.Render("Features:"))
		fmt.Printf("  %-12s %s\n", labelStyle.Render("View Status:"), proStyle.Render("✓"))
		fmt.Printf("  %-12s %s\n", labelStyle.Render("Memory (14 days):"), proStyle.Render("✓"))
		fmt.Printf("  %-12s %s\n", labelStyle.Render("Cloud Sync:"), freeStyle.Render("✗"))
		fmt.Printf("  %-12s %s\n", labelStyle.Render("Save Memory:"), freeStyle.Render("✗"))

		// Friendly prompt
		fmt.Println()
		fmt.Println(infoStyle.Render("  💡 Login to unlock more features:"))
		fmt.Println("    contextsync login")
		fmt.Println()
		fmt.Println("  Login benefits:")
		fmt.Println("    • Sync to 12+ AI tools (Free: 2)")
		fmt.Println("    • Cloud sync across devices")
		fmt.Println("    • Permanent memory storage")
		fmt.Println("    • Priority support")
		fmt.Println()
		return
	}

	// Logged in - show full info
	fmt.Println()
	fmt.Println(titleStyle.Render("Account:"))
	fmt.Printf("  %-12s %s\n", labelStyle.Render("Email:"), valueStyle.Render(config.GetAccountEmail()))

	// Initialize validator
	validator := license.NewValidator(config.GetServerURL())
	validator.SetIdentity(config.GetAccountID(), config.GetDeviceID())
	validator.SetDB(database)

	// License tier. GetFeatures validates with the server when possible.
	features := validator.GetFeatures()
	tier := validator.GetTier()
	isPaid := tier == "pro" || tier == "team"

	if isPaid {
		displayTier := "Pro"
		if tier == "team" {
			displayTier = "Team Pro"
		}
		fmt.Printf("  %-12s %s\n", labelStyle.Render("License:"), proStyle.Render(displayTier))
		if subType := validator.GetSubscriptionType(); subType != "" {
			fmt.Printf("  %-12s %s\n", labelStyle.Render("Plan:"), subType)
		}
		if exp := validator.GetExpiry(); exp != nil {
			days := int(time.Until(*exp).Hours() / 24)
			if days > 0 {
				fmt.Printf("  %-12s %s (%d days remaining)\n", labelStyle.Render("Expires:"), exp.Format("2006-01-02"), days)
			}
		}
	} else {
		fmt.Printf("  %-12s %s\n", labelStyle.Render("License:"), freeStyle.Render("Free (14 days)"))
	}

	// Configured tools
	var toolCount int
	database.DB().QueryRow("SELECT COUNT(*) FROM configured_tools").Scan(&toolCount)
	maxTools := features.MaxTools
	if isPaid {
		maxTools = 999 // Unlimited
	}

	fmt.Println()
	fmt.Println(titleStyle.Render("Tools:"))
	if isPaid {
		fmt.Printf("  %-12s %d configured (unlimited)\n", labelStyle.Render("Count:"), toolCount)
	} else {
		fmt.Printf("  %-12s %d / %d\n", labelStyle.Render("Count:"), toolCount, maxTools)
	}

	// Features
	fmt.Println()
	fmt.Println(titleStyle.Render("Features:"))

	checkIcon := func(enabled bool) string {
		if enabled {
			return proStyle.Render("✓")
		}
		return freeStyle.Render("✗")
	}

	fmt.Printf("  %-12s %s\n", labelStyle.Render("Cloud Sync:"), checkIcon(features.CanSync))
	fmt.Printf("  %-12s %s\n", labelStyle.Render("Save Memory:"), checkIcon(features.CanSaveMemory))
	fmt.Printf("  %-12s %s\n", labelStyle.Render("Retention:"), features.MemoryRetention)

	// Upgrade prompt for free users
	if tier == "free" {
		linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA")).Underline(true)
		fmt.Println()
		fmt.Println(titleStyle.Render("Plans:"))
		fmt.Printf("  %-12s $%d/mo\n", labelStyle.Render("Monthly:"), 19)
		fmt.Printf("  %-12s $%d/yr\n", labelStyle.Render("Yearly:"), 179)

		fmt.Println()
		fmt.Printf("  Upgrade: %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Render("contextsync upgrade"))
		fmt.Println("  Pricing: " + linkStyle.Render("https://contextsync.yangqing.one/pricing"))
	}

	fmt.Println()
}
