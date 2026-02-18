package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func tempBlacklistPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "blacklist.json")
}

func TestNewBlacklistStore(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	entries := bs.List()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestAddAndList(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	entry, err := bs.Add("path", "/usr/sbin/cupsd")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if entry.ID == "" {
		t.Error("expected non-empty ID")
	}
	if entry.Type != "path" {
		t.Errorf("expected type 'path', got %q", entry.Type)
	}
	if entry.Value != "/usr/sbin/cupsd" {
		t.Errorf("expected value '/usr/sbin/cupsd', got %q", entry.Value)
	}

	entries := bs.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestAddInvalidType(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	_, err = bs.Add("invalid", "value")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestAddInvalidPID(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	_, err = bs.Add("pid", "not-a-number")
	if err == nil {
		t.Error("expected error for invalid PID")
	}
}

func TestAddInvalidPattern(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	_, err = bs.Add("pattern", "[invalid")
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}

func TestRemove(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	entry, err := bs.Add("path", "/some/path")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	err = bs.Remove(entry.ID)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	entries := bs.List()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after remove, got %d", len(entries))
	}
}

func TestRemoveNotFound(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	err = bs.Remove("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestPersistence(t *testing.T) {
	path := tempBlacklistPath(t)

	// Create and add entries
	bs1, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	_, err = bs1.Add("path", "/usr/sbin/cupsd")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	_, err = bs1.Add("pattern", "^test-.*")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Reload from disk
	bs2, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore reload failed: %v", err)
	}

	entries := bs2.List()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after reload, got %d", len(entries))
	}
}

func TestAtomicWrite(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	_, err = bs.Add("path", "/test/path")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Verify the file exists and is valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty file")
	}
}

func TestIsBlacklistedBuiltinPaths(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	tests := []struct {
		exePath string
		args    []string
		want    bool
		desc    string
	}{
		{"/usr/libexec/some-daemon", nil, true, "system libexec"},
		{"/usr/lib/some-lib", nil, true, "system lib"},
		{"/private/var/something", nil, true, "private var"},
		{"/var/something", nil, true, "var"},
		{"/opt/X11/something", nil, true, "X11"},
		{"/Applications/MyApp.app/Contents/MacOS/MyApp", nil, false, "user application"},
		{"/Users/me/projects/server", nil, false, "user project"},
		{"/usr/local/bin/myapp", nil, false, "user-installed binary"},
	}

	for _, tt := range tests {
		got := bs.IsBlacklisted(tt.exePath, tt.args)
		if got != tt.want {
			t.Errorf("IsBlacklisted(%q) = %v, want %v (%s)", tt.exePath, got, tt.want, tt.desc)
		}
	}
}

func TestIsBlacklistedOwnExecutable(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	if !bs.IsBlacklisted("/usr/local/bin/nameport-daemon", nil) {
		t.Error("expected nameport-daemon to be blacklisted")
	}
	if !bs.IsBlacklisted("/usr/local/bin/nameport", nil) {
		t.Error("expected nameport to be blacklisted")
	}
}

func TestIsBlacklistedUserEntryPath(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	_, err = bs.Add("path", "/usr/local/bin/myapp")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if !bs.IsBlacklisted("/usr/local/bin/myapp", nil) {
		t.Error("expected user-blacklisted path to be blacklisted")
	}
	if bs.IsBlacklisted("/usr/local/bin/other", nil) {
		t.Error("expected non-blacklisted path to not be blacklisted")
	}
}

func TestIsBlacklistedUserEntryPattern(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	_, err = bs.Add("pattern", "^/opt/custom/.*")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if !bs.IsBlacklisted("/opt/custom/myapp", nil) {
		t.Error("expected pattern-matched path to be blacklisted")
	}
	if bs.IsBlacklisted("/opt/other/myapp", nil) {
		t.Error("expected non-matching path to not be blacklisted")
	}
}

func TestIsBlacklistedPID(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	_, err = bs.Add("pid", "12345")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if !bs.IsBlacklistedPID(12345) {
		t.Error("expected PID 12345 to be blacklisted")
	}
	if bs.IsBlacklistedPID(99999) {
		t.Error("expected PID 99999 to not be blacklisted")
	}
}

func TestIsBlacklistedInterpreterWithUserScript(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	// Python running a user script should not be blacklisted even though python
	// is in /usr/bin which could be a system path
	args := []string{"python3", "/Users/me/project/server.py"}
	if bs.IsBlacklisted("/usr/bin/python3", args) {
		t.Error("expected python running user script to not be blacklisted")
	}
}

func TestIsBlacklistedPatternOnArgs(t *testing.T) {
	path := tempBlacklistPath(t)
	bs, err := NewBlacklistStore(path)
	if err != nil {
		t.Fatalf("NewBlacklistStore failed: %v", err)
	}

	_, err = bs.Add("pattern", "test-server")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Pattern matches in args
	args := []string{"node", "test-server.js"}
	if !bs.IsBlacklisted("/usr/local/bin/node", args) {
		t.Error("expected pattern matching args to be blacklisted")
	}
}
