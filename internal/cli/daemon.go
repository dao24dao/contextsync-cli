package cli

import (
	"fmt"
	"time"

	"contextsync/internal/config"
	"contextsync/internal/daemon"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the ContextSync daemon",
	Long: `Manage the ContextSync background daemon.

The daemon automatically:
- Syncs rules to all configured AI tools when rules.md changes
- Syncs memories to cloud (Pro users)
- Runs in the background and starts automatically on login`,
}

var daemonRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "Run the daemon in foreground",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		runDaemon()
	},
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon service",
	Run: func(cmd *cobra.Command, args []string) {
		startDaemon()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon service",
	Run: func(cmd *cobra.Command, args []string) {
		stopDaemon()
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Run: func(cmd *cobra.Command, args []string) {
		daemonStatus()
	},
}

var daemonInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the daemon service",
	Run: func(cmd *cobra.Command, args []string) {
		installDaemon()
	},
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the daemon service",
	Run: func(cmd *cobra.Command, args []string) {
		uninstallDaemon()
	},
}

var daemonLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show daemon logs",
	Run: func(cmd *cobra.Command, args []string) {
		showDaemonLogs()
	},
}

func init() {
	daemonCmd.AddCommand(daemonRunCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonInstallCmd)
	daemonCmd.AddCommand(daemonUninstallCmd)
	daemonCmd.AddCommand(daemonLogsCmd)

	// For backward compatibility: `contextsync daemon` without subcommand runs in foreground
	daemonCmd.Run = func(cmd *cobra.Command, args []string) {
		runDaemon()
	}
}

func runDaemon() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))

	fmt.Println(titleStyle.Render("\n  ContextSync Daemon\n"))

	// Initialize
	if err := config.Init(); err != nil {
		fmt.Printf("  Failed to initialize config: %v\n", err)
		return
	}

	if err := initDatabase(); err != nil {
		fmt.Printf("  Failed to initialize database: %v\n", err)
		return
	}
	defer closeDatabase()

	// Create and run daemon
	d := daemon.New(database.DB(), validator, daemon.WithSyncInterval(5*time.Minute))

	fmt.Println("  Daemon running... (press Ctrl+C to stop)")
	fmt.Println()

	if err := d.Run(); err != nil {
		fmt.Printf("  Daemon error: %v\n", err)
	}
}

func startDaemon() {
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	svc := daemon.NewServiceManager()

	// Check if installed
	if !svc.IsInstalled() {
		fmt.Println(errorStyle.Render("  Daemon service not installed."))
		fmt.Println("\n  Run: contextsync daemon install")
		return
	}

	if err := svc.Start(); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("  Failed to start: %v", err)))
		return
	}

	fmt.Println(successStyle.Render("  Daemon started"))
}

func stopDaemon() {
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	svc := daemon.NewServiceManager()

	if err := svc.Stop(); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("  Failed to stop: %v", err)))
		return
	}

	fmt.Println(successStyle.Render("  Daemon stopped"))
}

func daemonStatus() {
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))

	svc := daemon.NewServiceManager()

	fmt.Println("\n  ContextSync Daemon Status\n")

	// Check if installed
	if !svc.IsInstalled() {
		fmt.Println(warnStyle.Render("  Not installed"))
		fmt.Println("\n  Run: contextsync daemon install")
		return
	}

	fmt.Println(infoStyle.Render("  Service: Installed"))

	// Check status
	status, err := svc.Status()
	if err != nil {
		fmt.Printf("  Status: Error - %v\n", err)
		return
	}

	if status.Running {
		fmt.Println(successStyle.Render("  Status: Running"))
		if status.PID > 0 {
			fmt.Printf("  PID: %d\n", status.PID)
		}
	} else {
		fmt.Println("  Status: Stopped")
	}

	fmt.Println()
}

func installDaemon() {
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

	fmt.Println("\n  Installing ContextSync daemon...\n")

	svc := daemon.NewServiceManager()

	// Check if already installed
	if svc.IsInstalled() {
		fmt.Println("  Daemon already installed.")
		fmt.Println("\n  To reinstall, run:")
		fmt.Println("    contextsync daemon uninstall")
		fmt.Println("    contextsync daemon install")
		return
	}

	// Install
	if err := svc.Install(); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("  Installation failed: %v", err)))
		return
	}

	fmt.Println(successStyle.Render("  Daemon service installed"))

	// Start the service
	if err := svc.Start(); err != nil {
		fmt.Println(infoStyle.Render(fmt.Sprintf("  Service installed but failed to start: %v", err)))
		fmt.Println("\n  Try starting manually: contextsync daemon start")
		return
	}

	fmt.Println(successStyle.Render("  Daemon started"))
	fmt.Println("\n  The daemon will automatically start on login and restart if crashed.")
}

func uninstallDaemon() {
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	svc := daemon.NewServiceManager()

	if !svc.IsInstalled() {
		fmt.Println("  Daemon is not installed.")
		return
	}

	if err := svc.Uninstall(); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("  Uninstallation failed: %v", err)))
		return
	}

	fmt.Println(successStyle.Render("  Daemon service uninstalled"))
}

func showDaemonLogs() {
	logs := daemon.ReadLogs(50)
	fmt.Println(logs)
}
