//go:build linux

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	systemdUnitName = "localhost-magic.service"
	systemdUnitDir  = "/etc/systemd/system"
)

// SystemdManager manages the localhost-magic daemon as a Linux systemd service.
type SystemdManager struct{}

// UnitPath returns the full path to the systemd unit file.
func (m *SystemdManager) UnitPath() string {
	return filepath.Join(systemdUnitDir, systemdUnitName)
}

// GenerateUnit generates the systemd unit file content for the given daemon binary path.
func GenerateUnit(daemonPath string) string {
	return fmt.Sprintf(`[Unit]
Description=localhost-magic daemon
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, daemonPath)
}

// Install writes the unit file and enables the service.
func (m *SystemdManager) Install(daemonPath string) error {
	absPath, err := filepath.Abs(daemonPath)
	if err != nil {
		return fmt.Errorf("resolving daemon path: %w", err)
	}

	unit := GenerateUnit(absPath)
	unitPath := m.UnitPath()

	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return fmt.Errorf("writing unit file to %s: %w", unitPath, err)
	}

	// Reload systemd to pick up the new unit
	cmd := exec.Command("systemctl", "daemon-reload")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s: %w", string(out), err)
	}

	// Enable the service
	cmd = exec.Command("systemctl", "enable", systemdUnitName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable: %s: %w", string(out), err)
	}

	return nil
}

// Uninstall disables the service and removes the unit file.
func (m *SystemdManager) Uninstall() error {
	// Stop if running
	_ = m.Stop()

	// Disable the service
	cmd := exec.Command("systemctl", "disable", systemdUnitName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl disable: %s: %w", string(out), err)
	}

	// Remove the unit file
	unitPath := m.UnitPath()
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file: %w", err)
	}

	// Reload systemd
	cmd = exec.Command("systemctl", "daemon-reload")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s: %w", string(out), err)
	}

	return nil
}

// Status checks whether the service is installed and running.
func (m *SystemdManager) Status() (ServiceStatus, error) {
	status := ServiceStatus{}

	// Check if unit file exists
	if _, err := os.Stat(m.UnitPath()); err == nil {
		status.Installed = true
	} else if !os.IsNotExist(err) {
		return status, fmt.Errorf("checking unit file: %w", err)
	}

	// Check if service is active
	cmd := exec.Command("systemctl", "is-active", systemdUnitName)
	out, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		status.Running = true
	}

	// Get main PID
	cmd = exec.Command("systemctl", "show", "-p", "MainPID", systemdUnitName)
	out, err = cmd.Output()
	if err == nil {
		line := strings.TrimSpace(string(out))
		if strings.HasPrefix(line, "MainPID=") {
			pidStr := strings.TrimPrefix(line, "MainPID=")
			fmt.Sscanf(pidStr, "%d", &status.PID)
		}
	}

	return status, nil
}

// Start starts the service via systemctl.
func (m *SystemdManager) Start() error {
	cmd := exec.Command("systemctl", "start", systemdUnitName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl start: %s: %w", string(out), err)
	}
	return nil
}

// Stop stops the service via systemctl.
func (m *SystemdManager) Stop() error {
	cmd := exec.Command("systemctl", "stop", systemdUnitName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl stop: %s: %w", string(out), err)
	}
	return nil
}
