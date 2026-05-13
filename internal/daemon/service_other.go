//go:build !darwin && !linux && !windows

package daemon

// NewServiceManager returns an unsupported service manager for other platforms
func NewServiceManager() ServiceManager {
	return &unsupportedService{}
}
