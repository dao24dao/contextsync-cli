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
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#111827"))
	proStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	freeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	fmt.Println(titleStyle.Render("\nContextSync Status\n"))

	// Version
	fmt.Printf("  %-12s %s\n", labelStyle.Render("Version:"), valueStyle.Render(version))

	// Check login status
	if !config.IsLoggedIn() {
		fmt.Println()
		fmt.Println(warnStyle.Render("  Not logged in"))
		fmt.Println("\n  Please login first:")
		fmt.Println("    contextsync login\n")
		return
	}

	// Account info
	fmt.Printf("  %-12s %s\n", labelStyle.Render("Account:"), valueStyle.Render(config.GetAccountEmail()))

	// Initialize validator
	validator := license.NewValidator(config.GetServerURL())
	ensureDatabase()
	validator.SetDB(database)

	// License tier
	tier := validator.GetTier()
	features := validator.GetFeatures()

	if tier == "pro" {
		fmt.Printf("  %-12s %s\n", labelStyle.Render("License:"), proStyle.Render("Pro"))
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
		fmt.Printf("  %-12s %s\n", labelStyle.Render("License:"), freeStyle.Render("Free"))
	}

	// Device info
	fmt.Printf("  %-12s %s\n", labelStyle.Render("Device ID:"), config.GetDeviceID())

	// Memory stats
	memRepo := getMemoryRepo()
	stats := memRepo.GetStats()

	fmt.Println()
	fmt.Println(titleStyle.Render("Storage:"))
	fmt.Printf("  %-12s %d memories\n", labelStyle.Render("Total:"), stats.Total)

	if stats.Expiring > 0 && tier == "free" {
		fmt.Printf("  %-12s %d expiring soon\n", labelStyle.Render("Warning:"), stats.Expiring)
	}

	// Configured tools
	var toolCount int
	database.DB().QueryRow("SELECT COUNT(*) FROM configured_tools").Scan(&toolCount)
	maxTools := features.MaxTools
	if tier == "pro" {
		maxTools = 999 // Unlimited
	}

	fmt.Println()
	fmt.Println(titleStyle.Render("Tools:"))
	if tier == "pro" {
		fmt.Printf("  %-12s %d configured (unlimited)\n", labelStyle.Render("Count:"), toolCount)
	} else {
		fmt.Printf("  %-12s %d / %d\n", labelStyle.Render("Count:"), toolCount, maxTools)
	}

	// Features
	fmt.Println()
	fmt.Println(titleStyle.Render("Features:"))

	checkIcon := func(enabled bool) string {
		if enabled {
			return proStyle.Render("ok")
		}
		return freeStyle.Render("x")
	}

	fmt.Printf("  %-12s %s\n", labelStyle.Render("Cloud Sync:"), checkIcon(features.CanSync))
	fmt.Printf("  %-12s %s\n", labelStyle.Render("Save Memory:"), checkIcon(features.CanSaveMemory))
	fmt.Printf("  %-12s %s\n", labelStyle.Render("Retention:"), features.MemoryRetention)

	// Upgrade prompt for free users
	if tier == "free" {
		fmt.Println()
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Render("  Upgrade to Pro:"))
		fmt.Println("    contextsync upgrade")
		fmt.Println()
		fmt.Println("  Plans: Monthly $9 | Quarterly $24 | Yearly $89")
	}

	fmt.Println()
}
