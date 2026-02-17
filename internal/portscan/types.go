// Package portscan discovers listening TCP sockets and their owning processes
package portscan

// Listener represents a process listening on a port
type Listener struct {
	Port    int
	PID     int
	ExePath string
	Cwd     string // Current working directory
	Args    []string
}
