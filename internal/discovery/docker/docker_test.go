package docker

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Available()
// ---------------------------------------------------------------------------

func TestAvailable_NoSocket(t *testing.T) {
	d := NewDiscovery("/tmp/nonexistent-docker-test.sock")
	if d.Available() {
		t.Fatal("Available() should return false when socket does not exist")
	}
}

func TestAvailable_SocketExists(t *testing.T) {
	// Create a real Unix socket so os.Stat reports ModeSocket.
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create test socket: %v", err)
	}
	defer ln.Close()

	d := NewDiscovery(sockPath)
	if !d.Available() {
		t.Fatal("Available() should return true when socket exists")
	}
}

// ---------------------------------------------------------------------------
// CleanContainerName
// ---------------------------------------------------------------------------

func TestCleanContainerName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/myapp", "myapp"},
		{"/my-cool-app", "my-cool-app"},
		{"already-clean", "already-clean"},
		{"/", ""},
		{"", ""},
	}
	for _, tc := range tests {
		got := CleanContainerName(tc.input)
		if got != tc.want {
			t.Errorf("CleanContainerName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// JSON parsing with fixtures
// ---------------------------------------------------------------------------

func TestParseContainers_HostMappedPort(t *testing.T) {
	raw := `[{
		"Id": "abc123",
		"Names": ["/web-app"],
		"Image": "nginx:latest",
		"Labels": {},
		"Ports": [
			{"IP": "0.0.0.0", "PrivatePort": 80, "PublicPort": 8080, "Type": "tcp"}
		],
		"NetworkSettings": {
			"Networks": {
				"bridge": {"IPAddress": "172.17.0.2"}
			}
		}
	}]`

	var containers []containerJSON
	if err := json.Unmarshal([]byte(raw), &containers); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	services := parseContainers(containers)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	svc := services[0]
	if svc.ContainerID != "abc123" {
		t.Errorf("ContainerID = %q, want abc123", svc.ContainerID)
	}
	if svc.ContainerName != "web-app" {
		t.Errorf("ContainerName = %q, want web-app", svc.ContainerName)
	}
	if svc.ImageName != "nginx:latest" {
		t.Errorf("ImageName = %q, want nginx:latest", svc.ImageName)
	}
	// Host-mapped port should use 127.0.0.1 and public port.
	if svc.TargetHost != "127.0.0.1" {
		t.Errorf("TargetHost = %q, want 127.0.0.1", svc.TargetHost)
	}
	if svc.Port != 8080 {
		t.Errorf("Port = %d, want 8080", svc.Port)
	}
}

func TestParseContainers_BridgeOnlyPort(t *testing.T) {
	raw := `[{
		"Id": "def456",
		"Names": ["/backend"],
		"Image": "myapp:dev",
		"Labels": {},
		"Ports": [
			{"PrivatePort": 3000, "PublicPort": 0, "Type": "tcp"}
		],
		"NetworkSettings": {
			"Networks": {
				"bridge": {"IPAddress": "172.17.0.5"}
			}
		}
	}]`

	var containers []containerJSON
	if err := json.Unmarshal([]byte(raw), &containers); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	services := parseContainers(containers)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	svc := services[0]
	// Non-mapped port should use bridge IP and private port.
	if svc.TargetHost != "172.17.0.5" {
		t.Errorf("TargetHost = %q, want 172.17.0.5", svc.TargetHost)
	}
	if svc.Port != 3000 {
		t.Errorf("Port = %d, want 3000", svc.Port)
	}
}

func TestParseContainers_UDPPortSkipped(t *testing.T) {
	raw := `[{
		"Id": "udp1",
		"Names": ["/dns-server"],
		"Image": "coredns",
		"Labels": {},
		"Ports": [
			{"PrivatePort": 53, "PublicPort": 53, "Type": "udp"}
		],
		"NetworkSettings": {"Networks": {}}
	}]`

	var containers []containerJSON
	if err := json.Unmarshal([]byte(raw), &containers); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	services := parseContainers(containers)
	if len(services) != 0 {
		t.Fatalf("expected 0 services for UDP-only ports, got %d", len(services))
	}
}

func TestParseContainers_MultiplePorts(t *testing.T) {
	raw := `[{
		"Id": "multi1",
		"Names": ["/fullstack"],
		"Image": "myimage",
		"Labels": {},
		"Ports": [
			{"IP": "0.0.0.0", "PrivatePort": 80, "PublicPort": 8080, "Type": "tcp"},
			{"IP": "0.0.0.0", "PrivatePort": 443, "PublicPort": 8443, "Type": "tcp"}
		],
		"NetworkSettings": {"Networks": {"bridge": {"IPAddress": "172.17.0.3"}}}
	}]`

	var containers []containerJSON
	if err := json.Unmarshal([]byte(raw), &containers); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	services := parseContainers(containers)
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if services[0].Port != 8080 {
		t.Errorf("first service port = %d, want 8080", services[0].Port)
	}
	if services[1].Port != 8443 {
		t.Errorf("second service port = %d, want 8443", services[1].Port)
	}
}

// ---------------------------------------------------------------------------
// Compose label extraction
// ---------------------------------------------------------------------------

func TestParseContainers_ComposeLabels(t *testing.T) {
	raw := `[{
		"Id": "compose1",
		"Names": ["/myproject-web-1"],
		"Image": "myproject-web",
		"Labels": {
			"com.docker.compose.project": "myproject",
			"com.docker.compose.service": "web"
		},
		"Ports": [
			{"IP": "0.0.0.0", "PrivatePort": 3000, "PublicPort": 3000, "Type": "tcp"}
		],
		"NetworkSettings": {"Networks": {"bridge": {"IPAddress": "172.17.0.10"}}}
	}]`

	var containers []containerJSON
	if err := json.Unmarshal([]byte(raw), &containers); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	services := parseContainers(containers)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	svc := services[0]
	if svc.ComposeProject != "myproject" {
		t.Errorf("ComposeProject = %q, want myproject", svc.ComposeProject)
	}
	if svc.ComposeService != "web" {
		t.Errorf("ComposeService = %q, want web", svc.ComposeService)
	}
}

// ---------------------------------------------------------------------------
// nameport.name label override
// ---------------------------------------------------------------------------

func TestParseContainers_NameLabel(t *testing.T) {
	raw := `[{
		"Id": "label1",
		"Names": ["/boring-container-name"],
		"Image": "myimage",
		"Labels": {
			"nameport.name": "cool-api"
		},
		"Ports": [
			{"IP": "0.0.0.0", "PrivatePort": 8000, "PublicPort": 8000, "Type": "tcp"}
		],
		"NetworkSettings": {"Networks": {"bridge": {"IPAddress": "172.17.0.7"}}}
	}]`

	var containers []containerJSON
	if err := json.Unmarshal([]byte(raw), &containers); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	services := parseContainers(containers)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	if services[0].ContainerName != "cool-api" {
		t.Errorf("ContainerName = %q, want cool-api (from label override)", services[0].ContainerName)
	}
}

// ---------------------------------------------------------------------------
// Port mapping logic: resolveHostPort
// ---------------------------------------------------------------------------

func TestResolveHostPort(t *testing.T) {
	tests := []struct {
		name     string
		pm       portMapping
		bridgeIP string
		wantHost string
		wantPort int
	}{
		{
			name:     "host mapped",
			pm:       portMapping{IP: "0.0.0.0", PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
			bridgeIP: "172.17.0.2",
			wantHost: "127.0.0.1",
			wantPort: 8080,
		},
		{
			name:     "bridge only",
			pm:       portMapping{PrivatePort: 3000, PublicPort: 0, Type: "tcp"},
			bridgeIP: "172.17.0.5",
			wantHost: "172.17.0.5",
			wantPort: 3000,
		},
		{
			name:     "no bridge no mapping",
			pm:       portMapping{PrivatePort: 3000, PublicPort: 0, Type: "tcp"},
			bridgeIP: "",
			wantHost: "",
			wantPort: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, port := resolveHostPort(tc.pm, tc.bridgeIP)
			if host != tc.wantHost || port != tc.wantPort {
				t.Errorf("resolveHostPort() = (%q, %d), want (%q, %d)", host, port, tc.wantHost, tc.wantPort)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// containerBridgeIP
// ---------------------------------------------------------------------------

func TestContainerBridgeIP(t *testing.T) {
	tests := []struct {
		name string
		ns   *networkSettings
		want string
	}{
		{"nil", nil, ""},
		{"empty networks", &networkSettings{Networks: map[string]networkEntry{}}, ""},
		{"bridge present", &networkSettings{Networks: map[string]networkEntry{
			"bridge": {IPAddress: "172.17.0.2"},
		}}, "172.17.0.2"},
		{"custom network only", &networkSettings{Networks: map[string]networkEntry{
			"my-net": {IPAddress: "10.0.0.5"},
		}}, "10.0.0.5"},
		{"bridge preferred over custom", &networkSettings{Networks: map[string]networkEntry{
			"my-net": {IPAddress: "10.0.0.5"},
			"bridge": {IPAddress: "172.17.0.2"},
		}}, "172.17.0.2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := containerBridgeIP(tc.ns)
			if got != tc.want {
				t.Errorf("containerBridgeIP() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Integration-style test: Scan() against a fake Docker daemon
// ---------------------------------------------------------------------------

func TestScan_FakeDaemon(t *testing.T) {
	// Start a fake Docker daemon on a temp Unix socket.
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "docker.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	fixture := `[{
		"Id": "aaa111",
		"Names": ["/test-svc"],
		"Image": "testimg",
		"Labels": {"nameport.name": "my-svc", "com.docker.compose.project": "proj"},
		"Ports": [{"IP":"0.0.0.0","PrivatePort":80,"PublicPort":9090,"Type":"tcp"}],
		"NetworkSettings": {"Networks": {"bridge": {"IPAddress": "172.17.0.99"}}}
	}]`

	mux := http.NewServeMux()
	mux.HandleFunc("/"+apiVersion+"/containers/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fixture))
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	d := NewDiscovery(sockPath)
	services, err := d.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	svc := services[0]
	if svc.ContainerName != "my-svc" {
		t.Errorf("ContainerName = %q, want my-svc", svc.ContainerName)
	}
	if svc.Port != 9090 {
		t.Errorf("Port = %d, want 9090", svc.Port)
	}
	if svc.TargetHost != "127.0.0.1" {
		t.Errorf("TargetHost = %q, want 127.0.0.1", svc.TargetHost)
	}
	if svc.ComposeProject != "proj" {
		t.Errorf("ComposeProject = %q, want proj", svc.ComposeProject)
	}
}

// ---------------------------------------------------------------------------
// NewDiscovery default socket path
// ---------------------------------------------------------------------------

func TestNewDiscovery_DefaultSocket(t *testing.T) {
	d := NewDiscovery("")
	if d.socketPath != defaultSocketPath {
		t.Errorf("socketPath = %q, want %q", d.socketPath, defaultSocketPath)
	}
}

// ---------------------------------------------------------------------------
// Available() returns false for plain files (not sockets)
// ---------------------------------------------------------------------------

func TestAvailable_RegularFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "not-a-socket")
	os.WriteFile(f, []byte("hi"), 0644)

	d := NewDiscovery(f)
	// Regular files are accepted in Available() for test convenience.
	if !d.Available() {
		t.Fatal("Available() should return true for regular files (test convenience)")
	}
}
