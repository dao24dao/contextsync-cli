//go:build linux

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	systemdServiceName = "contextsync.service"
)

// NewServiceManager returns the Linux service manager
func NewServiceManager() ServiceManager {
	return &linuxService{}
}

type linuxService struct{}

func (s *linuxService) getServicePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", systemdServiceName)
}

func (s *linuxService) Install() error {
	servicePath := s.getServicePath()
	exePath := GetExecutablePath()

	// Ensure directory exists
	serviceDir := filepath.Dir(servicePath)
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd directory: %w", err)
	}

	service := fmt.Sprintf(`[Unit]
Description=ContextSync Daemon
After=network.target

[Service]
Type=simple
ExecStart=%s daemon --run
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, exePath)

	if err := os.WriteFile(servicePath, []byte(service), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd daemon
	exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

func (s *linuxService) Uninstall() error {
	servicePath := s.getServicePath()

	// Stop the service first
	s.Stop()

	// Disable the service
	exec.Command("systemctl", "--user", "disable", systemdServiceName).Run()

	// Remove service file
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd daemon
	exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

func (s *linuxService) Start() error {
	servicePath := s.getServicePath()

	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		return fmt.Errorf("service not installed")
	}

	// Enable and start the service
	if err := exec.Command("systemctl", "--user", "enable", systemdServiceName).Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "start", systemdServiceName).Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func (s *linuxService) Stop() error {
	exec.Command("systemctl", "--user", "stop", systemdServiceName).Run()
	return nil
}

func (s *linuxService) Status() (Status, error) {
	cmd := exec.Command("systemctl", "--user", "is-active", systemdServiceName)
	output, err := cmd.CombinedOutput()

	status := Status{}
	if err != nil {
		status.Running = false
		return status, nil
	}

	status.Running = string(output) == "active\n"

	// Get PID if running
	if status.Running {
		cmd = exec.Command("systemctl", "--user", "show", "--property=MainPID", systemdServiceName)
		output, _ = cmd.CombinedOutput()
		var pid int
		fmt.Sscanf(string(output), "MainPID=%d", &pid)
		status.PID = pid
	}

	return status, nil
}

func (s *linuxService) IsInstalled() bool {
	servicePath := s.getServicePath()
	_, err := os.Stat(servicePath)
	return err == nil
}
