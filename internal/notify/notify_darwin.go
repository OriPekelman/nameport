//go:build darwin

package notify

import (
	"fmt"
	"os/exec"
)

// DarwinNotifier sends desktop notifications on macOS via osascript.
// Notifications are informational only — clicking them has no effect.
// The URL field is ignored on macOS because osascript does not support
// click-to-open and would otherwise activate Script Editor.
type DarwinNotifier struct{}

// Send delivers a notification on macOS.
func (d *DarwinNotifier) Send(n Notification) error {
	script := fmt.Sprintf(
		`display notification %q with title %q`,
		n.Message, n.Title,
	)
	return exec.Command("osascript", "-e", script).Run()
}

// IsAvailable always returns true on macOS because osascript is always present.
func (d *DarwinNotifier) IsAvailable() bool {
	return true
}

// NewPlatformNotifier returns the platform-specific notifier for macOS.
func NewPlatformNotifier() Notifier {
	return &DarwinNotifier{}
}
