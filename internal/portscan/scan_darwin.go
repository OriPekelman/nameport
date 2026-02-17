//go:build darwin

package portscan

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Scan discovers all listening TCP sockets and their owning processes on macOS
func Scan() ([]Listener, error) {
	// Use lsof to find listening TCP sockets
	// lsof -nP -iTCP -sTCP:LISTEN -F pn
	// Output format: p<pid>\nn<address:port>\n...
	cmd := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pn")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lsof failed: %w", err)
	}

	// Parse lsof output
	portToPID := make(map[int]int)
	var currentPID int

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}

		prefix := line[0]
		value := line[1:]

		switch prefix {
		case 'p':
			// PID line
			pid, err := strconv.Atoi(value)
			if err == nil {
				currentPID = pid
			}
		case 'n':
			// Network address line: "127.0.0.1:3000" or "*:3000" or "[::1]:3000"
			port := parsePort(value)
			if port > 0 && currentPID > 0 {
				portToPID[port] = currentPID
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse lsof output: %w", err)
	}

	// Build listener list
	var listeners []Listener
	for port, pid := range portToPID {
		exePath, cwd, args, err := getProcessInfo(pid)
		if err != nil {
			// Process may have exited, skip
			continue
		}

		listeners = append(listeners, Listener{
			Port:    port,
			PID:     pid,
			ExePath: exePath,
			Cwd:     cwd,
			Args:    args,
		})
	}

	return listeners, nil
}

// parsePort extracts the port number from lsof address format
// Handles: "127.0.0.1:3000", "*:3000", "[::1]:3000", "[fe80::1%lo0]:3000"
func parsePort(addr string) int {
	// Find the last colon (handles IPv6 addresses with colons)
	idx := strings.LastIndex(addr, ":")
	if idx == -1 {
		return 0
	}

	portStr := addr[idx+1:]

	// Remove any trailing info (like (LISTEN))
	if parenIdx := strings.Index(portStr, "("); parenIdx != -1 {
		portStr = portStr[:parenIdx]
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}

	return port
}

// getProcessInfo gets the executable path, cwd and command line for a PID on macOS
func getProcessInfo(pid int) (string, string, []string, error) {
	// Use lsof to get executable path and cwd
	// lsof -p <pid> -F n
	cmd := exec.Command("lsof", "-p", strconv.Itoa(pid), "-F", "n")
	output, err := cmd.Output()
	if err != nil {
		return "", "", nil, fmt.Errorf("lsof failed for pid %d: %w", pid, err)
	}

	var exePath, cwd string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ntxt") {
			// This is the text file (executable)
			exePath = line[4:] // Remove 'ntxt' prefix
		} else if strings.HasPrefix(line, "ncwd") {
			// This is the current working directory
			cwd = line[4:] // Remove 'ncwd' prefix
		}
	}

	if exePath == "" {
		// Fallback: try using ps to get command
		exePath = getCommandFromPS(pid)
	}

	// Get command line arguments using ps
	args := getCommandLine(pid)

	return exePath, cwd, args, nil
}

// getCommandFromPS gets the command path using ps
func getCommandFromPS(pid int) string {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getCommandLine gets the full command line for a process
func getCommandLine(pid int) []string {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	args := strings.TrimSpace(string(output))
	if args == "" {
		return nil
	}

	// Simple split - this is imperfect but works for most cases
	return strings.Fields(args)
}

// ResolveExecutablePath attempts to get the absolute path to the executable
// On macOS, this resolves symlinks and finds the real binary
func ResolveExecutablePath(cmd string) string {
	// Try to find the full path
	if filepath.IsAbs(cmd) {
		return cmd
	}

	// Check common locations
	paths := []string{
		"/usr/bin/" + cmd,
		"/usr/local/bin/" + cmd,
		"/opt/homebrew/bin/" + cmd,
		"/opt/local/bin/" + cmd,
		"/bin/" + cmd,
		"/sbin/" + cmd,
	}

	for _, p := range paths {
		if _, err := exec.Command("test", "-f", p).Output(); err == nil {
			return p
		}
	}

	// Use which command
	out, err := exec.Command("which", cmd).Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	return cmd
}
