//go:build darwin

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	launchAgentLabel = "com.contextsync.daemon"
)

// NewServiceManager returns the macOS service manager
func NewServiceManager() ServiceManager {
	return &macOSService{}
}

type macOSService struct{}

func (s *macOSService) getLaunchAgentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func (s *macOSService) Install() error {
	plistPath := s.getLaunchAgentPath()
	exePath := GetExecutablePath()
	logPath := GetDaemonLogPath()

	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	os.MkdirAll(logDir, 0755)

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>daemon</string>
        <string>--run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin</string>
    </dict>
</dict>
</plist>
`, launchAgentLabel, exePath, logPath, logPath)

	// Ensure LaunchAgents directory exists
	launchDir := filepath.Dir(plistPath)
	if err := os.MkdirAll(launchDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	return nil
}

func (s *macOSService) Uninstall() error {
	plistPath := s.getLaunchAgentPath()

	// Stop the service first
	s.Stop()

	// Remove plist file
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	return nil
}

func (s *macOSService) Start() error {
	plistPath := s.getLaunchAgentPath()

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return fmt.Errorf("service not installed")
	}

	// Use launchctl to load the service
	cmd := exec.Command("launchctl", "load", plistPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}

	return nil
}

func (s *macOSService) Stop() error {
	plistPath := s.getLaunchAgentPath()

	// Use launchctl to unload the service
	cmd := exec.Command("launchctl", "unload", plistPath)
	cmd.Run() // Ignore error if not loaded

	return nil
}

func (s *macOSService) Status() (Status, error) {
	// Check if service is running using launchctl list
	cmd := exec.Command("launchctl", "list", launchAgentLabel)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Status{Running: false}, nil
	}

	// Parse output to get PID
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, launchAgentLabel) {
			fields := strings.Fields(line)
			if len(fields) >= 1 && fields[0] != "-" {
				// First field is PID
				var pid int
				fmt.Sscanf(fields[0], "%d", &pid)
				return Status{Running: true, PID: pid}, nil
			}
		}
	}

	return Status{Running: true}, nil
}

func (s *macOSService) IsInstalled() bool {
	plistPath := s.getLaunchAgentPath()
	_, err := os.Stat(plistPath)
	return err == nil
}
