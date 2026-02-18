// Package docker discovers services running in Docker containers via the Docker Engine API.
package docker

import (
	"context"
	"net"
	"net/http"
)

// newUnixTransport creates an http.Transport that dials via a Unix domain socket.
func newUnixTransport(socketPath string) *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
}
