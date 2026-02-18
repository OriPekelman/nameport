//go:build darwin

package system

import (
	"strings"
	"testing"
)

func TestGeneratePlist(t *testing.T) {
	daemonPath := "/usr/local/bin/nameport-daemon"
	plist := GeneratePlist(daemonPath)

	// Verify it is valid XML-ish plist
	if !strings.Contains(plist, `<?xml version="1.0"`) {
		t.Error("plist should contain XML declaration")
	}
	if !strings.Contains(plist, `<!DOCTYPE plist`) {
		t.Error("plist should contain DOCTYPE")
	}

	// Verify label
	if !strings.Contains(plist, "<string>com.nameport.daemon</string>") {
		t.Error("plist should contain the service label")
	}

	// Verify daemon path
	if !strings.Contains(plist, "<string>"+daemonPath+"</string>") {
		t.Errorf("plist should contain daemon path %s", daemonPath)
	}

	// Verify RunAtLoad
	if !strings.Contains(plist, "<key>RunAtLoad</key>") {
		t.Error("plist should contain RunAtLoad key")
	}
	if !strings.Contains(plist, "<true/>") {
		t.Error("plist should have RunAtLoad set to true")
	}

	// Verify KeepAlive
	if !strings.Contains(plist, "<key>KeepAlive</key>") {
		t.Error("plist should contain KeepAlive key")
	}

	// Verify log paths
	if !strings.Contains(plist, "<key>StandardOutPath</key>") {
		t.Error("plist should contain StandardOutPath")
	}
	if !strings.Contains(plist, "<key>StandardErrorPath</key>") {
		t.Error("plist should contain StandardErrorPath")
	}
	if !strings.Contains(plist, "<string>/var/log/nameport.log</string>") {
		t.Error("plist should point logs to /var/log/nameport.log")
	}
}

func TestGeneratePlistDifferentPaths(t *testing.T) {
	paths := []string{
		"/opt/nameport/bin/daemon",
		"/home/user/go/bin/nameport",
		"/usr/bin/nameport-daemon",
	}

	for _, p := range paths {
		plist := GeneratePlist(p)
		if !strings.Contains(plist, "<string>"+p+"</string>") {
			t.Errorf("plist should contain daemon path %s", p)
		}
	}
}

func TestLaunchdManagerPlistPath(t *testing.T) {
	m := &LaunchdManager{}
	expected := "/Library/LaunchDaemons/com.nameport.daemon.plist"
	if m.PlistPath() != expected {
		t.Errorf("PlistPath() = %q, want %q", m.PlistPath(), expected)
	}
}

func TestNewServiceManagerReturnLaunchd(t *testing.T) {
	mgr := NewServiceManager()
	if _, ok := mgr.(*LaunchdManager); !ok {
		t.Error("NewServiceManager() on darwin should return *LaunchdManager")
	}
}
