//go:build linux

package notify

import (
	"os/exec"
)

// LinuxNotifier sends desktop notifications on Linux using notify-send.
type LinuxNotifier struct{}

// Send delivers a notification via notify-send. If a URL is present,
// it's appended to the message body since notify-send doesn't reliably
// support click actions across all desktop environments.
func (l *LinuxNotifier) Send(n Notification) error {
	msg := n.Message
	if n.URL != "" {
		msg = msg + "\n" + n.URL
	}
	return exec.Command("notify-send",
		"--app-name", "localhost-magic",
		n.Title, msg,
	).Run()
}

// IsAvailable reports whether notify-send is installed.
func (l *LinuxNotifier) IsAvailable() bool {
	_, err := exec.LookPath("notify-send")
	return err == nil
}

// NewPlatformNotifier returns the platform-specific notifier for Linux.
func NewPlatformNotifier() Notifier {
	return &LinuxNotifier{}
}
