package notify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Fatal("DefaultConfigPath returned empty string")
	}
	if filepath.Base(path) != "notify.json" {
		t.Errorf("expected filename notify.json, got %s", filepath.Base(path))
	}
}

func TestLoadConfigFileNotExist(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected default config to be enabled")
	}
	if len(cfg.EventFilter) != len(AllEvents()) {
		t.Errorf("expected %d events, got %d", len(AllEvents()), len(cfg.EventFilter))
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.json")

	cfg := DefaultConfig()
	cfg.Enabled = false
	cfg.EventFilter[EventServiceOffline] = false

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.Enabled {
		t.Error("expected loaded config to be disabled")
	}
	if loaded.EventFilter[EventServiceOffline] {
		t.Error("expected service_offline to be disabled")
	}
	if !loaded.EventFilter[EventServiceDiscovered] {
		t.Error("expected service_discovered to still be enabled")
	}
}

func TestSaveConfigCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	path := filepath.Join(dir, "notify.json")

	if err := SaveConfig(path, DefaultConfig()); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0666); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
