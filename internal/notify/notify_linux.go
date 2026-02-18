//go:build linux

package notify

import (
	"os/exec"
)

// LinuxNotifier sends desktop notifications on Linux using notify-send.
type LinuxNotifier struct{}

// Send delivers a notification via notify-send.
func (l *LinuxNotifier) Send(n Notification) error {
	return exec.Command("notify-send", n.Title, n.Message).Run()
}

// IsAvailable reports whether notify-send is installed.
func (l *LinuxNotifier) IsAvailable() bool {
	_, err := exec.LookPath("notify-send")
	return err == nil
}
