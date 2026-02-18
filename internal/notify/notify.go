// Package notify provides desktop notification support for nameport.
// It emits notifications on service discovery events, TLS certificate expiry,
// and peer connection changes. Platform-specific implementations live in
// notify_darwin.go, notify_linux.go, and notify_other.go.
package notify

// EventType represents a category of notification event.
type EventType string

const (
	EventServiceDiscovered EventType = "service_discovered"
	EventServiceOffline    EventType = "service_offline"
	EventServiceRenamed    EventType = "service_renamed"
	EventCertExpiring      EventType = "cert_expiring"
	EventPeerConnected     EventType = "peer_connected"
	EventPeerDisconnected  EventType = "peer_disconnected"
)

// AllEvents returns a slice of every defined EventType.
func AllEvents() []EventType {
	return []EventType{
		EventServiceDiscovered,
		EventServiceOffline,
		EventServiceRenamed,
		EventCertExpiring,
		EventPeerConnected,
		EventPeerDisconnected,
	}
}

// Notification holds the data for a single desktop notification.
type Notification struct {
	Event   EventType `json:"event"`
	Title   string    `json:"title"`
	Message string    `json:"message"`
	URL     string    `json:"url,omitempty"`
}

// Notifier is the interface that platform-specific notification backends
// must implement.
type Notifier interface {
	// Send delivers a notification to the desktop.
	Send(n Notification) error
	// IsAvailable reports whether the notification backend is functional
	// on the current system.
	IsAvailable() bool
}

// Config controls which notification events are enabled.
type Config struct {
	Enabled     bool                 `json:"enabled"`
	EventFilter map[EventType]bool   `json:"event_filter"`
}

// DefaultConfig returns a Config with all events enabled.
func DefaultConfig() Config {
	filter := make(map[EventType]bool)
	for _, e := range AllEvents() {
		filter[e] = true
	}
	return Config{
		Enabled:     true,
		EventFilter: filter,
	}
}
