package system

// ServiceStatus represents the current state of the daemon service.
type ServiceStatus struct {
	Installed bool
	Running   bool
	PID       int
}

// ServiceManager provides an interface for installing, uninstalling,
// and managing the nameport daemon as a system service.
type ServiceManager interface {
	// Install registers the daemon as a system service using the given daemon binary path.
	Install(daemonPath string) error
	// Uninstall removes the daemon from the system service manager.
	Uninstall() error
	// Status returns the current service status.
	Status() (ServiceStatus, error)
	// Start starts the daemon service.
	Start() error
	// Stop stops the daemon service.
	Stop() error
}
