package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ServiceManager manages the daemon as a system service
type ServiceManager interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Status() (Status, error)
	IsInstalled() bool
}

type Status struct {
	Running   bool
	PID       int
	StartTime string
}

// GetExecutablePath returns the path to the contextsync executable
func GetExecutablePath() string {
	// First, try to find in PATH
	if path, err := exec.LookPath("contextsync"); err == nil {
		return path
	}

	// Fallback: use the current executable
	if exe, err := os.Executable(); err == nil {
		return exe
	}

	// Last resort: assume it's in /usr/local/bin
	return "/usr/local/bin/contextsync"
}

// GetLogPath returns the daemon log file path
func GetDaemonLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".contextsync", "logs", "daemon.log")
}

// NewServiceManager is implemented in platform-specific files
// service_darwin.go, service_linux.go, service_windows.go

// unsupportedService is a fallback for unsupported platforms
type unsupportedService struct{}

func (s *unsupportedService) Install() error {
	return fmt.Errorf("daemon service not supported on this platform")
}

func (s *unsupportedService) Uninstall() error {
	return fmt.Errorf("daemon service not supported on this platform")
}

func (s *unsupportedService) Start() error {
	return fmt.Errorf("daemon service not supported on this platform")
}

func (s *unsupportedService) Stop() error {
	return fmt.Errorf("daemon service not supported on this platform")
}

func (s *unsupportedService) Status() (Status, error) {
	return Status{}, fmt.Errorf("daemon service not supported on this platform")
}

func (s *unsupportedService) IsInstalled() bool {
	return false
}
