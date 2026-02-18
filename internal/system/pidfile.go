package system

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// WritePID writes the given process ID to the specified file path.
func WritePID(path string, pid int) error {
	data := []byte(strconv.Itoa(pid) + "\n")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing PID file %s: %w", path, err)
	}
	return nil
}

// ReadPID reads and returns the process ID from the specified file path.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading PID file %s: %w", path, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parsing PID from %s: %w", path, err)
	}

	return pid, nil
}

// RemovePID removes the PID file at the specified path.
func RemovePID(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing PID file %s: %w", path, err)
	}
	return nil
}

// IsRunning checks whether the process identified by the PID file is currently running.
// Returns false if the PID file does not exist or the process is not running.
func IsRunning(path string) bool {
	pid, err := ReadPID(path)
	if err != nil {
		return false
	}

	// Signal 0 checks for process existence without sending a signal.
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}
