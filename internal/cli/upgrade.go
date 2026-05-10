package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"contextsync/internal/config"
	"contextsync/internal/integrations"
	"contextsync/internal/license"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade to ContextSync Pro",
	Long: `Upgrade to ContextSync Pro for unlimited access to all AI coding tools.

Features unlocked with Pro:
  • All 12+ AI tools (Claude Code, Cursor, GitHub Copilot, Windsurf, Gemini, etc.)
  • Permanent memory retention
  • Unlimited memory storage
  • Cloud sync across devices
  • Priority support`,
	Run: func(cmd *cobra.Command, args []string) {
		runUpgrade()
	},
}

func runUpgrade() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	proStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	freeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))

	fmt.Println(titleStyle.Render("\nUpgrade to ContextSync Pro\n"))

	// Check if already Pro
	ensureDatabase()
	if validator != nil && validator.IsPro() {
		fmt.Println(proStyle.Render("  You already have a Pro subscription!"))
		subType := validator.GetSubscriptionType()
		if subType != "" {
			fmt.Printf("\n  Plan: %s\n", subType)
		}
		fmt.Println("\n  Run 'contextsync status' for details.\n")
		return
	}

	// Check if logged in
	if !config.IsLoggedIn() {
		fmt.Println(warnStyle.Render("  Not logged in"))
		fmt.Println("\n  Please login first:")
		fmt.Println("    contextsync login\n")
		return
	}

	// Show current account
	fmt.Printf("  Account: %s\n\n", config.GetAccountEmail())

	// Show tool count
	toolCount := integrations.GetToolCount()
	fmt.Printf("  Supported Tools: %d+\n\n", toolCount)

	// Free vs Pro comparison
	fmt.Println(freeStyle.Render("  Free Tier:"))
	fmt.Println("    - 2 tools only")
	fmt.Println("    - 14-day memory retention")
	fmt.Println("    - Read-only memory access")
	fmt.Println("    - No cloud sync\n")

	fmt.Println(proStyle.Render("  Pro Tier:"))
	fmt.Println("    - All 12+ AI tools")
	fmt.Println("    - Permanent memory retention")
	fmt.Println("    - Unlimited memory storage")
	fmt.Println("    - Cloud sync across devices (3 devices)")
	fmt.Println("    - Priority support\n")

	// Subscription plans
	fmt.Println(titleStyle.Render("Subscription Plans:\n"))

	for _, plan := range license.SubscriptionPlans {
		savingsText := ""
		if plan.Savings != "" {
			savingsText = fmt.Sprintf(" (%s)", plan.Savings)
		}
		fmt.Printf("  %-12s $%d %s\n", plan.Name+":", plan.Price/100, savingsText)
	}

	// Build pricing URL
	serverURL := config.GetServerURL()
	accountID := config.GetAccountID()
	email := config.GetAccountEmail()

	pricingURL := serverURL + "/pricing"
	if accountID != "" || email != "" {
		pricingURL += "?"
		if accountID != "" {
			pricingURL += "account_id=" + accountID
		}
		if email != "" {
			if accountID != "" {
				pricingURL += "&"
			}
			pricingURL += "email=" + email
		}
	}

	fmt.Println()
	fmt.Println(titleStyle.Render("Opening pricing page in your browser..."))
	fmt.Println()
	fmt.Printf("  URL: %s\n\n", pricingURL)

	// Warning about email
	fmt.Println(warnStyle.Render("Important:"))
	fmt.Println("  Please use the same email for payment:")
	fmt.Printf("  %s\n\n", email)

	// Open browser
	if err := openBrowser(pricingURL); err != nil {
		fmt.Println("  Could not open browser automatically.")
		fmt.Println("  Please visit the URL above manually.")
	}

	fmt.Println()
	fmt.Println("After payment:")
	fmt.Println("  1. Your subscription will be activated automatically")
	fmt.Println("  2. Run 'contextsync status' to verify")
	fmt.Println()
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
