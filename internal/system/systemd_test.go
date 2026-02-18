//go:build linux

package system

import (
	"strings"
	"testing"
)

func TestGenerateUnit(t *testing.T) {
	daemonPath := "/usr/local/bin/localhost-magic-daemon"
	unit := GenerateUnit(daemonPath)

	// Verify [Unit] section
	if !strings.Contains(unit, "[Unit]") {
		t.Error("unit should contain [Unit] section")
	}
	if !strings.Contains(unit, "Description=localhost-magic daemon") {
		t.Error("unit should contain Description")
	}
	if !strings.Contains(unit, "After=network.target") {
		t.Error("unit should contain After=network.target")
	}

	// Verify [Service] section
	if !strings.Contains(unit, "[Service]") {
		t.Error("unit should contain [Service] section")
	}
	if !strings.Contains(unit, "Type=simple") {
		t.Error("unit should contain Type=simple")
	}
	if !strings.Contains(unit, "ExecStart="+daemonPath) {
		t.Errorf("unit should contain ExecStart=%s", daemonPath)
	}
	if !strings.Contains(unit, "Restart=always") {
		t.Error("unit should contain Restart=always")
	}

	// Verify [Install] section
	if !strings.Contains(unit, "[Install]") {
		t.Error("unit should contain [Install] section")
	}
	if !strings.Contains(unit, "WantedBy=multi-user.target") {
		t.Error("unit should contain WantedBy=multi-user.target")
	}
}

func TestGenerateUnitDifferentPaths(t *testing.T) {
	paths := []string{
		"/opt/localhost-magic/bin/daemon",
		"/home/user/go/bin/localhost-magic",
		"/usr/bin/localhost-magic-daemon",
	}

	for _, p := range paths {
		unit := GenerateUnit(p)
		if !strings.Contains(unit, "ExecStart="+p) {
			t.Errorf("unit should contain ExecStart=%s", p)
		}
	}
}

func TestSystemdManagerUnitPath(t *testing.T) {
	m := &SystemdManager{}
	expected := "/etc/systemd/system/localhost-magic.service"
	if m.UnitPath() != expected {
		t.Errorf("UnitPath() = %q, want %q", m.UnitPath(), expected)
	}
}

func TestNewServiceManagerReturnSystemd(t *testing.T) {
	mgr := NewServiceManager()
	if _, ok := mgr.(*SystemdManager); !ok {
		t.Error("NewServiceManager() on linux should return *SystemdManager")
	}
}
