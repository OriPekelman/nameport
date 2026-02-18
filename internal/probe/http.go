package probe

import (
	"bufio"
	"crypto/tls"
	"net"
	"strconv"
	"strings"
	"time"
)

// Protocol represents the detected protocol of a service
type Protocol int

const (
	ProtoNone  Protocol = iota // Not an HTTP service
	ProtoHTTP                  // Plain HTTP
	ProtoHTTPS                 // HTTPS (TLS)
)

// String returns the string representation of a Protocol
func (p Protocol) String() string {
	switch p {
	case ProtoHTTP:
		return "http"
	case ProtoHTTPS:
		return "https"
	default:
		return "none"
	}
}

// IsHTTP checks if the service on the given host:port speaks HTTP
// Sends a simple GET request and checks for HTTP response
func IsHTTP(host string, port int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// Try to connect with timeout
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Set read/write timeout
	conn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	// Send a simple HTTP request
	request := "GET / HTTP/1.0\r\n\r\n"
	_, err = conn.Write([]byte(request))
	if err != nil {
		return false
	}

	// Read response
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	// Check if response starts with "HTTP/"
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "HTTP/")
}

// IsHTTPS checks if the service on the given host:port speaks HTTPS
// Attempts a TLS handshake and sends an HTTP request over TLS
func IsHTTPS(host string, port int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// Try to connect with timeout
	rawConn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	defer rawConn.Close()

	// Set deadline for the entire TLS handshake + request
	rawConn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	// Attempt TLS handshake (skip verify since these are local services)
	tlsConn := tls.Client(rawConn, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err := tlsConn.Handshake(); err != nil {
		return false
	}

	// Send a simple HTTP request over TLS
	request := "GET / HTTP/1.0\r\n\r\n"
	_, err = tlsConn.Write([]byte(request))
	if err != nil {
		return false
	}

	// Read response
	reader := bufio.NewReader(tlsConn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	// Check if response starts with "HTTP/"
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "HTTP/")
}

// DetectProtocol attempts to detect the protocol of a service.
// It first tries HTTPS (TLS handshake), then falls back to plain HTTP.
func DetectProtocol(host string, port int) Protocol {
	// Try HTTPS first
	if IsHTTPS(host, port) {
		return ProtoHTTPS
	}

	// Fall back to plain HTTP
	if IsHTTP(host, port) {
		return ProtoHTTP
	}

	return ProtoNone
}

// ProbeResult contains detailed information about an HTTP probe
type ProbeResult struct {
	IsHTTP   bool
	IsHTTPS  bool
	Protocol Protocol
	Response string
}

// Probe performs a detailed HTTP probe and returns the response status line
func Probe(host string, port int) ProbeResult {
	// Detect protocol
	proto := DetectProtocol(host, port)

	if proto == ProtoHTTPS {
		// Get the HTTPS response line for details
		response := probeHTTPS(host, port)
		return ProbeResult{
			IsHTTP:   false,
			IsHTTPS:  true,
			Protocol: ProtoHTTPS,
			Response: response,
		}
	}

	if proto == ProtoHTTP {
		// Get the HTTP response line for details
		response := probeHTTP(host, port)
		return ProbeResult{
			IsHTTP:   true,
			IsHTTPS:  false,
			Protocol: ProtoHTTP,
			Response: response,
		}
	}

	return ProbeResult{
		IsHTTP:   false,
		IsHTTPS:  false,
		Protocol: ProtoNone,
	}
}

// probeHTTP sends an HTTP request and returns the response status line
func probeHTTP(host string, port int) string {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return ""
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	request := "GET / HTTP/1.0\r\nHost: localhost\r\n\r\n"
	_, err = conn.Write([]byte(request))
	if err != nil {
		return ""
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}

	return strings.TrimSpace(line)
}

// probeHTTPS sends an HTTP request over TLS and returns the response status line
func probeHTTPS(host string, port int) string {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	rawConn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return ""
	}
	defer rawConn.Close()

	rawConn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	tlsConn := tls.Client(rawConn, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err := tlsConn.Handshake(); err != nil {
		return ""
	}

	request := "GET / HTTP/1.0\r\nHost: localhost\r\n\r\n"
	_, err = tlsConn.Write([]byte(request))
	if err != nil {
		return ""
	}

	reader := bufio.NewReader(tlsConn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}

	return strings.TrimSpace(line)
}
