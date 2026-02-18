//go:build darwin

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	launchdLabel    = "com.localhost-magic.daemon"
	launchdPlistDir = "/Library/LaunchDaemons"
	launchdLogPath  = "/var/log/localhost-magic.log"
)

// LaunchdManager manages the localhost-magic daemon as a macOS launchd service.
type LaunchdManager struct{}

// PlistPath returns the full path to the plist file.
func (m *LaunchdManager) PlistPath() string {
	return filepath.Join(launchdPlistDir, launchdLabel+".plist")
}

// GeneratePlist generates the launchd plist XML for the given daemon binary path.
func GeneratePlist(daemonPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`, launchdLabel, daemonPath, launchdLogPath, launchdLogPath)
}

// Install writes the plist and loads it via launchctl.
func (m *LaunchdManager) Install(daemonPath string) error {
	absPath, err := filepath.Abs(daemonPath)
	if err != nil {
		return fmt.Errorf("resolving daemon path: %w", err)
	}

	plist := GeneratePlist(absPath)
	plistPath := m.PlistPath()

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("writing plist to %s: %w", plistPath, err)
	}

	cmd := exec.Command("launchctl", "load", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %s: %w", string(out), err)
	}

	return nil
}

// Uninstall unloads and removes the plist.
func (m *LaunchdManager) Uninstall() error {
	plistPath := m.PlistPath()

	cmd := exec.Command("launchctl", "unload", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl unload: %s: %w", string(out), err)
	}

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plist: %w", err)
	}

	return nil
}

// Status checks whether the service is installed and running.
func (m *LaunchdManager) Status() (ServiceStatus, error) {
	status := ServiceStatus{}

	// Check if plist exists
	if _, err := os.Stat(m.PlistPath()); err == nil {
		status.Installed = true
	} else if !os.IsNotExist(err) {
		return status, fmt.Errorf("checking plist: %w", err)
	}

	// Check launchctl list for our label
	cmd := exec.Command("launchctl", "list", launchdLabel)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Service not loaded
		return status, nil
	}

	status.Running = true

	// Parse PID from launchctl list output
	// Output format includes lines like:
	// "PID" = 12345;
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "\"PID\"") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				pidStr := strings.TrimSpace(parts[1])
				pidStr = strings.TrimSuffix(pidStr, ";")
				pidStr = strings.TrimSpace(pidStr)
				if pid, err := strconv.Atoi(pidStr); err == nil {
					status.PID = pid
				}
			}
		}
	}

	return status, nil
}

// Start starts the service via launchctl.
func (m *LaunchdManager) Start() error {
	cmd := exec.Command("launchctl", "start", launchdLabel)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl start: %s: %w", string(out), err)
	}
	return nil
}

// Stop stops the service via launchctl.
func (m *LaunchdManager) Stop() error {
	cmd := exec.Command("launchctl", "stop", launchdLabel)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl stop: %s: %w", string(out), err)
	}
	return nil
}
