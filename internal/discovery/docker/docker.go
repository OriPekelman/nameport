package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	defaultSocketPath = "/var/run/docker.sock"
	apiVersion        = "v1.43"
)

// ContainerService represents a service discovered from a running Docker container.
type ContainerService struct {
	ContainerID    string
	ContainerName  string
	ImageName      string
	Port           int
	TargetHost     string
	Labels         map[string]string
	ComposeProject string
	ComposeService string
}

// Discovery scans the Docker daemon for running containers.
type Discovery struct {
	socketPath string
	client     *http.Client
}

// NewDiscovery creates a Discovery that communicates with the Docker daemon
// over the given Unix socket path. If socketPath is empty, the default
// /var/run/docker.sock is used.
func NewDiscovery(socketPath string) *Discovery {
	if socketPath == "" {
		socketPath = defaultSocketPath
	}
	return &Discovery{
		socketPath: socketPath,
		client: &http.Client{
			Transport: newUnixTransport(socketPath),
		},
	}
}

// Available reports whether the Docker socket exists and is accessible.
func (d *Discovery) Available() bool {
	info, err := os.Stat(d.socketPath)
	if err != nil {
		return false
	}
	// Accept regular files (for tests) and sockets.
	return info.Mode().Type() == os.ModeSocket || info.Mode().IsRegular()
}

// Scan queries the Docker daemon for running containers and returns a
// ContainerService for every exposed port it finds.
func (d *Discovery) Scan() ([]ContainerService, error) {
	resp, err := d.client.Get("http://localhost/" + apiVersion + "/containers/json")
	if err != nil {
		return nil, fmt.Errorf("docker api request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading docker response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker api returned status %d: %s", resp.StatusCode, string(body))
	}

	var containers []containerJSON
	if err := json.Unmarshal(body, &containers); err != nil {
		return nil, fmt.Errorf("parsing docker response: %w", err)
	}

	return parseContainers(containers), nil
}

// --- Docker Engine API JSON types (subset) ---

type containerJSON struct {
	ID              string            `json:"Id"`
	Names           []string          `json:"Names"`
	Image           string            `json:"Image"`
	Labels          map[string]string `json:"Labels"`
	Ports           []portMapping     `json:"Ports"`
	NetworkSettings *networkSettings  `json:"NetworkSettings"`
}

type portMapping struct {
	IP          string `json:"IP"`
	PrivatePort int    `json:"PrivatePort"`
	PublicPort  int    `json:"PublicPort"`
	Type        string `json:"Type"`
}

type networkSettings struct {
	Networks map[string]networkEntry `json:"Networks"`
}

type networkEntry struct {
	IPAddress string `json:"IPAddress"`
}

// --- parsing helpers ---

// CleanContainerName strips the leading "/" from Docker container names.
func CleanContainerName(name string) string {
	return strings.TrimPrefix(name, "/")
}

// parseContainers converts raw Docker API container data into ContainerService
// entries. A container with multiple port mappings produces multiple entries.
func parseContainers(containers []containerJSON) []ContainerService {
	var services []ContainerService
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = CleanContainerName(c.Names[0])
		}

		composeProject := c.Labels["com.docker.compose.project"]
		composeService := c.Labels["com.docker.compose.service"]

		bridgeIP := containerBridgeIP(c.NetworkSettings)

		for _, p := range c.Ports {
			if p.Type != "tcp" {
				continue
			}

			host, port := resolveHostPort(p, bridgeIP)
			if port == 0 {
				continue
			}

			svc := ContainerService{
				ContainerID:    c.ID,
				ContainerName:  name,
				ImageName:      c.Image,
				Port:           port,
				TargetHost:     host,
				Labels:         c.Labels,
				ComposeProject: composeProject,
				ComposeService: composeService,
			}

			// Override name from label if present.
			if labelName, ok := c.Labels["localhost-magic.name"]; ok && labelName != "" {
				svc.ContainerName = labelName
			}

			services = append(services, svc)
		}
	}
	return services
}

// resolveHostPort determines the target host and port for a container port
// mapping. Host-mapped ports (PublicPort != 0) use 127.0.0.1; otherwise the
// container's bridge network IP is used with the private port.
func resolveHostPort(p portMapping, bridgeIP string) (string, int) {
	if p.PublicPort != 0 {
		return "127.0.0.1", p.PublicPort
	}
	if bridgeIP != "" && p.PrivatePort != 0 {
		return bridgeIP, p.PrivatePort
	}
	return "", 0
}

// containerBridgeIP returns the IP address from the container's bridge
// network, or the first available network IP if bridge is not found.
func containerBridgeIP(ns *networkSettings) string {
	if ns == nil || len(ns.Networks) == 0 {
		return ""
	}
	// Prefer the "bridge" network.
	if entry, ok := ns.Networks["bridge"]; ok && entry.IPAddress != "" {
		return entry.IPAddress
	}
	// Fallback: first non-empty IP.
	for _, entry := range ns.Networks {
		if entry.IPAddress != "" {
			return entry.IPAddress
		}
	}
	return ""
}
