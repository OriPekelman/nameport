//go:build darwin

package notify

import (
	"fmt"
	"os/exec"
)

// DarwinNotifier sends desktop notifications on macOS.
// Prefers terminal-notifier (supports click-to-open URL), falls back to osascript.
type DarwinNotifier struct {
	hasTerminalNotifier bool
}

// Send delivers a notification on macOS. If the notification has a URL and
// terminal-notifier is available, clicking the notification opens that URL.
// Otherwise falls back to osascript (no click-to-open support).
func (d *DarwinNotifier) Send(n Notification) error {
	if d.hasTerminalNotifier {
		return d.sendTerminalNotifier(n)
	}
	return d.sendOsascript(n)
}

func (d *DarwinNotifier) sendTerminalNotifier(n Notification) error {
	args := []string{
		"-title", n.Title,
		"-message", n.Message,
		"-group", "localhost-magic",
		"-sender", "com.apple.Safari", // Use Safari icon instead of Terminal
	}
	if n.URL != "" {
		args = append(args, "-open", n.URL)
	}
	return exec.Command("terminal-notifier", args...).Run()
}

func (d *DarwinNotifier) sendOsascript(n Notification) error {
	// Show the URL in subtitle so user can see it.
	// Note: Clicking an osascript notification opens Script Editor (the sender app).
	// There's no way around this with plain osascript, which is why
	// terminal-notifier is preferred. We at least show the URL for reference.
	subtitle := ""
	if n.URL != "" {
		subtitle = n.URL
	}

	var script string
	if subtitle != "" {
		script = fmt.Sprintf(
			`display notification %q with title %q subtitle %q`,
			n.Message, n.Title, subtitle,
		)
	} else {
		script = fmt.Sprintf(
			`display notification %q with title %q`,
			n.Message, n.Title,
		)
	}

	return exec.Command("osascript", "-e", script).Run()
}

// IsAvailable always returns true on macOS because osascript is always present.
func (d *DarwinNotifier) IsAvailable() bool {
	return true
}

// NewPlatformNotifier returns the platform-specific notifier for macOS.
// If terminal-notifier is installed (brew install terminal-notifier),
// notifications support click-to-open URLs. Otherwise falls back to
// osascript which shows the URL as subtitle text.
func NewPlatformNotifier() Notifier {
	_, err := exec.LookPath("terminal-notifier")
	return &DarwinNotifier{
		hasTerminalNotifier: err == nil,
	}
}
