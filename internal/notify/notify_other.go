//go:build !darwin && !linux

package notify

// OtherNotifier is a no-op notification backend for unsupported platforms.
type OtherNotifier struct{}

// Send is a no-op and always returns nil.
func (o *OtherNotifier) Send(n Notification) error {
	return nil
}

// IsAvailable always returns false on unsupported platforms.
func (o *OtherNotifier) IsAvailable() bool {
	return false
}

// NewPlatformNotifier returns the platform-specific notifier for unsupported platforms.
func NewPlatformNotifier() Notifier {
	return &OtherNotifier{}
}
