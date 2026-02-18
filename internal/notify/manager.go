package notify

import "fmt"

// Manager coordinates notification dispatch through a Notifier backend,
// filtering events according to Config.
type Manager struct {
	notifier Notifier
	config   Config
}

// NewManager creates a Manager with the given config and platform notifier.
func NewManager(config Config, notifier Notifier) *Manager {
	return &Manager{
		notifier: notifier,
		config:   config,
	}
}

// Notify sends a notification if the manager is enabled and the event type
// passes the config filter.
func (m *Manager) Notify(n Notification) error {
	if !m.config.Enabled {
		return nil
	}
	if allowed, exists := m.config.EventFilter[n.Event]; exists && !allowed {
		return nil
	}
	return m.notifier.Send(n)
}

// ServiceDiscovered sends a notification that a new service has been found.
func (m *Manager) ServiceDiscovered(name, port string) error {
	return m.Notify(Notification{
		Event:   EventServiceDiscovered,
		Title:   "Service Discovered",
		Message: fmt.Sprintf("%s is now available on port %s", name, port),
		URL:     fmt.Sprintf("http://%s.localhost", name),
	})
}

// ServiceOffline sends a notification that a service has gone offline.
func (m *Manager) ServiceOffline(name string) error {
	return m.Notify(Notification{
		Event:   EventServiceOffline,
		Title:   "Service Offline",
		Message: fmt.Sprintf("%s is no longer available", name),
	})
}

// ServiceRenamed sends a notification that a service has been renamed.
func (m *Manager) ServiceRenamed(oldName, newName string) error {
	return m.Notify(Notification{
		Event:   EventServiceRenamed,
		Title:   "Service Renamed",
		Message: fmt.Sprintf("%s has been renamed to %s", oldName, newName),
		URL:     fmt.Sprintf("http://%s.localhost", newName),
	})
}
