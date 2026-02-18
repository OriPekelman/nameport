//go:build linux

package system

// NewServiceManager returns a platform-appropriate ServiceManager.
// On Linux, this returns a SystemdManager.
func NewServiceManager() ServiceManager {
	return &SystemdManager{}
}
