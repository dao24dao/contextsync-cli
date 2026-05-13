//go:build windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	taskName = "ContextSyncDaemon"
)

// NewServiceManager returns the Windows service manager
func NewServiceManager() ServiceManager {
	return &windowsService{}
}

type windowsService struct{}

func (s *windowsService) Install() error {
	exePath := GetExecutablePath()
	logPath := GetDaemonLogPath()

	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	os.MkdirAll(logDir, 0755)

	// Create a scheduled task that runs at logon
	// Using schtasks command
	cmd := exec.Command("schtasks", "/create",
		"/tn", taskName,
		"/tr", fmt.Sprintf(`"%s" daemon --run`, exePath),
		"/sc", "onlogon",
		"/rl", "limited", // Run with limited privileges (user level)
		"/f", // Force overwrite if exists
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create scheduled task: %w\n%s", err, string(output))
	}

	return nil
}

func (s *windowsService) Uninstall() error {
	// Stop the task first
	s.Stop()

	// Delete the scheduled task
	cmd := exec.Command("schtasks", "/delete", "/tn", taskName, "/f")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "The system cannot find the file specified") {
		return fmt.Errorf("failed to delete scheduled task: %w\n%s", err, string(output))
	}

	return nil
}

func (s *windowsService) Start() error {
	// Check if task exists
	cmd := exec.Command("schtasks", "/query", "/tn", taskName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("service not installed")
	}

	// Run the task
	cmd = exec.Command("schtasks", "/run", "/tn", taskName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start task: %w\n%s", err, string(output))
	}

	return nil
}

func (s *windowsService) Stop() error {
	cmd := exec.Command("schtasks", "/end", "/tn", taskName)
	cmd.Run() // Ignore error if not running
	return nil
}

func (s *windowsService) Status() (Status, error) {
	cmd := exec.Command("schtasks", "/query", "/tn", taskName, "/v", "/fo", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Status{Running: false}, nil
	}

	// Parse output to check status
	status := Status{}
	outputStr := string(output)

	if strings.Contains(outputStr, "Running") {
		status.Running = true
	} else if strings.Contains(outputStr, "Ready") {
		status.Running = false
	}

	return status, nil
}

func (s *windowsService) IsInstalled() bool {
	cmd := exec.Command("schtasks", "/query", "/tn", taskName)
	return cmd.Run() == nil
}
