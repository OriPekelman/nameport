package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadPID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	expectedPID := 12345
	if err := WritePID(path, expectedPID); err != nil {
		t.Fatalf("WritePID() error: %v", err)
	}

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID() error: %v", err)
	}

	if pid != expectedPID {
		t.Errorf("ReadPID() = %d, want %d", pid, expectedPID)
	}
}

func TestReadPIDNonexistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.pid")

	_, err := ReadPID(path)
	if err == nil {
		t.Error("ReadPID() should return error for nonexistent file")
	}
}

func TestReadPIDInvalidContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pid")

	if err := os.WriteFile(path, []byte("not-a-number\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadPID(path)
	if err == nil {
		t.Error("ReadPID() should return error for invalid PID content")
	}
}

func TestRemovePID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	if err := WritePID(path, 99999); err != nil {
		t.Fatalf("WritePID() error: %v", err)
	}

	if err := RemovePID(path); err != nil {
		t.Fatalf("RemovePID() error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("PID file should have been removed")
	}
}

func TestRemovePIDNonexistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.pid")

	// Should not error on nonexistent file
	if err := RemovePID(path); err != nil {
		t.Errorf("RemovePID() should not error on nonexistent file: %v", err)
	}
}

func TestIsRunningCurrentProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	// Write our own PID — we know this process is running
	if err := WritePID(path, os.Getpid()); err != nil {
		t.Fatalf("WritePID() error: %v", err)
	}

	if !IsRunning(path) {
		t.Error("IsRunning() should return true for current process")
	}
}

func TestIsRunningNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.pid")

	if IsRunning(path) {
		t.Error("IsRunning() should return false for nonexistent PID file")
	}
}

func TestIsRunningDeadProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	// Use a PID that is very unlikely to be running
	// PID 99999999 should not exist on most systems
	if err := WritePID(path, 99999999); err != nil {
		t.Fatalf("WritePID() error: %v", err)
	}

	if IsRunning(path) {
		t.Error("IsRunning() should return false for dead process")
	}
}

func TestWritePIDOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	if err := WritePID(path, 111); err != nil {
		t.Fatalf("WritePID() error: %v", err)
	}

	if err := WritePID(path, 222); err != nil {
		t.Fatalf("WritePID() error on overwrite: %v", err)
	}

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID() error: %v", err)
	}

	if pid != 222 {
		t.Errorf("ReadPID() = %d, want 222 after overwrite", pid)
	}
}
