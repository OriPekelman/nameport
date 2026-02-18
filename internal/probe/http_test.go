package probe

import (
	"fmt"
	"net"
	"net/http"
	"testing"
)

func TestIsHTTP_PlainHTTP(t *testing.T) {
	// Start a plain HTTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "hello")
		}),
	}
	go server.Serve(listener)
	defer server.Close()

	if !IsHTTP("127.0.0.1", port) {
		t.Errorf("IsHTTP should return true for plain HTTP server on port %d", port)
	}
}

func TestIsHTTPS_PlainHTTP(t *testing.T) {
	// Start a plain HTTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "hello")
		}),
	}
	go server.Serve(listener)
	defer server.Close()

	if IsHTTPS("127.0.0.1", port) {
		t.Errorf("IsHTTPS should return false for plain HTTP server on port %d", port)
	}
}

func TestDetectProtocol_NonListeningPort(t *testing.T) {
	// Use a port that nothing is listening on
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close() // Close immediately so nothing is listening

	proto := DetectProtocol("127.0.0.1", port)
	if proto != ProtoNone {
		t.Errorf("DetectProtocol should return ProtoNone for non-listening port, got %v", proto)
	}
}

func TestDetectProtocol_PlainHTTP(t *testing.T) {
	// Start a plain HTTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "hello")
		}),
	}
	go server.Serve(listener)
	defer server.Close()

	proto := DetectProtocol("127.0.0.1", port)
	if proto != ProtoHTTP {
		t.Errorf("DetectProtocol should return ProtoHTTP for plain HTTP server, got %v", proto)
	}
}

func TestProtocol_String(t *testing.T) {
	tests := []struct {
		proto    Protocol
		expected string
	}{
		{ProtoNone, "none"},
		{ProtoHTTP, "http"},
		{ProtoHTTPS, "https"},
	}

	for _, tt := range tests {
		if got := tt.proto.String(); got != tt.expected {
			t.Errorf("Protocol(%d).String() = %q, want %q", tt.proto, got, tt.expected)
		}
	}
}
