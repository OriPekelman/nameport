//go:build darwin

package notify

import (
	"fmt"
	"os/exec"
)

// DarwinNotifier sends desktop notifications on macOS using osascript.
type DarwinNotifier struct{}

// Send delivers a notification via osascript "display notification".
func (d *DarwinNotifier) Send(n Notification) error {
	script := fmt.Sprintf(`display notification %q with title %q`, n.Message, n.Title)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// IsAvailable always returns true on macOS because osascript is always present.
func (d *DarwinNotifier) IsAvailable() bool {
	return true
}
