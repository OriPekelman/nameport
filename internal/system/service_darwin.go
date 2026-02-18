//go:build darwin

package system

// NewServiceManager returns a platform-appropriate ServiceManager.
// On macOS, this returns a LaunchdManager.
func NewServiceManager() ServiceManager {
	return &LaunchdManager{}
}
