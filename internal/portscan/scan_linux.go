//go:build linux

package portscan

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Scan discovers all listening TCP sockets and their owning processes
func Scan() ([]Listener, error) {
	// Parse /proc/net/tcp to get socket inodes
	inodes, err := parseTCPFile("/proc/net/tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to parse /proc/net/tcp: %w", err)
	}

	// Also check IPv6
	ipv6Inodes, err := parseTCPFile("/proc/net/tcp6")
	if err == nil {
		for port, inode := range ipv6Inodes {
			if _, exists := inodes[port]; !exists {
				inodes[port] = inode
			}
		}
	}

	// Map inodes to PIDs
	pidMap, err := mapInodesToPIDs(inodes)
	if err != nil {
		return nil, fmt.Errorf("failed to map inodes to PIDs: %w", err)
	}

	// Build listener list
	var listeners []Listener
	for port, pid := range pidMap {
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

// parseTCPFile parses /proc/net/tcp or /proc/net/tcp6
// Returns map of port -> inode
func parseTCPFile(path string) (map[int]uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[int]uint64)
	scanner := bufio.NewScanner(file)

	// Skip header line
	if !scanner.Scan() {
		return result, nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		// Check if state is LISTEN (0x0A)
		if fields[3] != "0A" {
			continue
		}

		// Parse local address (field 1): "0100007F:0050" = 127.0.0.1:80
		localAddr := fields[1]
		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 {
			continue
		}

		portHex := parts[1]
		port, err := strconv.ParseInt(portHex, 16, 32)
		if err != nil {
			continue
		}

		// Parse inode (field 9)
		inodeStr := fields[9]
		if inodeStr == "0" {
			continue
		}

		inode, err := strconv.ParseUint(inodeStr, 10, 64)
		if err != nil {
			continue
		}

		result[int(port)] = inode
	}

	return result, scanner.Err()
}

// mapInodesToPIDs scans /proc to find which PIDs own the given inodes
func mapInodesToPIDs(inodes map[int]uint64) (map[int]int, error) {
	result := make(map[int]int)

	// Scan /proc for all processes
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // Not a PID directory
		}

		// Scan /proc/<pid>/fd/ for socket symlinks
		fdDir := filepath.Join("/proc", entry.Name(), "fd")
		fdEntries, err := os.ReadDir(fdDir)
		if err != nil {
			continue // Can't read, probably permission denied
		}

		for _, fdEntry := range fdEntries {
			fdPath := filepath.Join(fdDir, fdEntry.Name())
			link, err := os.Readlink(fdPath)
			if err != nil {
				continue
			}

			// Check if it's a socket
			if !strings.HasPrefix(link, "socket:[") {
				continue
			}

			// Extract inode from "socket:[12345]"
			inodeStr := strings.TrimPrefix(link, "socket:[")
			inodeStr = strings.TrimSuffix(inodeStr, "]")
			inode, err := strconv.ParseUint(inodeStr, 10, 64)
			if err != nil {
				continue
			}

			// Check if this inode matches any of our listening sockets
			for port, listenInode := range inodes {
				if inode == listenInode {
					result[port] = pid
					break
				}
			}
		}
	}

	return result, nil
}

// getProcessInfo reads /proc/<pid>/exe, /proc/<pid>/cwd and /proc/<pid>/cmdline
func getProcessInfo(pid int) (string, string, []string, error) {
	pidStr := strconv.Itoa(pid)

	// Read exe path (resolves symlinks)
	exePath, err := os.Readlink(filepath.Join("/proc", pidStr, "exe"))
	if err != nil {
		return "", "", nil, err
	}

	// Read cwd
	cwd, err := os.Readlink(filepath.Join("/proc", pidStr, "cwd"))
	if err != nil {
		cwd = "" // CWD might not be available
	}

	// Read cmdline
	cmdlinePath := filepath.Join("/proc", pidStr, "cmdline")
	data, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return exePath, cwd, nil, nil // Return exe and cwd even if cmdline fails
	}

	// cmdline is null-separated
	args := strings.Split(string(data), "\x00")
	// Remove empty last element
	if len(args) > 0 && args[len(args)-1] == "" {
		args = args[:len(args)-1]
	}

	return exePath, cwd, args, nil
}
