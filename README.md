# localhost-magic

**Automatic `.localhost` DNS names for every HTTP service on your machine.**

Start any local server, and it's instantly reachable at a human-friendly URL. No config files, no `/etc/hosts` edits, no DNS servers. Just run your service and open `http://myapp.localhost`.

![Dashboard](images/dashboard.png)

![commandline](images/commandline.png)

## What It Does

localhost-magic is a daemon + CLI that watches your machine for HTTP services, names them, and reverse-proxies them through port 80 with `.localhost` domains.

```bash
$ cd ~/projects/myapp && npm start     # listening on :3000
# → http://myapp.localhost             (auto-discovered, auto-named)

$ cd ~/work/api && flask run           # listening on :5000
# → http://api.localhost               (named from directory)

$ cd ~/projects/myapp && npm run dev   # listening on :3001
# → http://myapp-1.localhost           (collision handled)
```

**It also handles services you don't run yourself** -- macOS apps, Docker containers, and remote machines all get names:

```bash
$ localhost-magic list
NAME                    TARGET              PID     COMMAND
ollama.localhost        127.0.0.1:11434     1033    /Applications/Ollama.app/...
dropbox.localhost       127.0.0.1:17600     48354   /Applications/Dropbox.app/...
api.localhost           127.0.0.1:3000      62138   node server.js
neverssl.localhost      34.223.124.45:80    0       manual   # remote proxy
```

## Quick Start

```bash
# Build
make

# Start the daemon (requires root for port 80)
sudo ./localhost-magic-daemon

# Start any local HTTP server
cd ~/projects/myapp && python3 -m http.server 3000

# Open in browser -- it just works
open http://myapp.localhost

# See all discovered services
./localhost-magic list

# View the web dashboard
open http://localhost/
```

## Features

### Discovery & Proxying
- **Automatic service discovery** -- scans for listening TCP ports every 2 seconds
- **HTTP verification** -- only proxies services that actually speak HTTP
- **Smart naming** -- 17 built-in rules extract names from project directories, macOS app bundles, script paths, and working directories
- **Collision handling** -- `myapp.localhost`, `myapp-1.localhost`, `myapp-2.localhost`
- **Remote target proxying** -- proxy to Docker containers, VMs, or machines on your LAN
- **Docker container detection** -- auto-discovers containers with exposed ports
- **No DNS server needed** -- `.localhost` is an IANA-reserved TLD that browsers resolve to `127.0.0.1`

### Management
- **Web dashboard** at `http://localhost/` with real-time health status, rename, keep, and blacklist controls
- **CLI** for all operations: `list`, `rename`, `keep`, `add`, `remove`, `blacklist`, `rules`, `notify`
- **Persistent names** -- custom renames survive daemon restarts
- **Keep mode** -- pin services in the dashboard even when they're offline
- **Persistent blacklist** -- block services by PID, executable path, or regex pattern
- **Customizable naming rules** -- data-driven JSON rules with user overrides and priority ordering

### Notifications
- **Desktop notifications** for service discovered, offline, and renamed events (macOS and Linux)
- **Per-event filtering** -- enable/disable individual notification types
- **Persistent config** at `~/.config/localhost-magic/notify.json`

### Security
- **Two-tier TLS certificate authority** -- Ed25519 root CA with ECDSA intermediate, ready for local HTTPS
- **Domain policy enforcement** -- CA only issues certs for safe TLDs (`.localhost`, `.test`, `.internal`), blocks all IANA public TLDs
- **systemd/launchd integration** -- auto-start support for production-like setups

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Browser       │────▶│  localhost-magic │────▶│  Your Service   │
│  (myapp.local)  │     │  (reverse proxy  │     │  (port 3000)    │
│                 │◀────│   on port 80)    │◀────│                 │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                               │
                               ▼
                        ┌──────────────────┐
                        │ Port Scanner     │
                        │ (Linux: /proc)   │
                        │ (macOS: lsof)    │
                        └──────────────────┘
```

## Building

Requires Go 1.21+.

### Using Make (Recommended)

```bash
# Build for current platform
make

# Build for Linux (from macOS or Linux)
make build-linux

# Clean build artifacts
make clean
```

### Manual Build

```bash
# Build daemon
go build -o localhost-magic-daemon ./cmd/daemon

# Build CLI
go build -o localhost-magic ./cmd/cli
```

## Usage

### Start the Daemon

The daemon must run as root to bind port 80:

```bash
sudo ./localhost-magic-daemon
```

Optional: specify custom config path:
```bash
sudo ./localhost-magic-daemon /path/to/services.json
```

### Manage Services via CLI

List all discovered services:
```bash
./localhost-magic list
```

Rename a service:
```bash
./localhost-magic rename myapp.localhost api.localhost
```

Toggle keep status (persist service even when not running):
```bash
./localhost-magic keep myapp.localhost         # Enable keep
./localhost-magic keep myapp.localhost false   # Disable keep
```

Add a manual service entry (for services not currently running):
```bash
./localhost-magic add staging.localhost 8080
```

Add a service targeting a remote host (Docker container, another machine on the LAN, etc.):
```bash
./localhost-magic add myapp.localhost 192.168.0.1:3000
./localhost-magic add docker-app.localhost 172.17.0.2:8080
```

Blacklist services:
```bash
./localhost-magic blacklist pid 12345                    # By PID
./localhost-magic blacklist path /usr/sbin/cupsd         # By executable path
./localhost-magic blacklist pattern "^localhost-magic"   # By regex pattern
./localhost-magic blacklist list                         # List all user blacklist entries
./localhost-magic blacklist remove <id>                  # Remove a blacklist entry
```

Manage naming rules:
```bash
./localhost-magic rules list                             # Show active rules with priority
./localhost-magic rules export                           # Export rules as JSON
./localhost-magic rules import my-rules.json             # Import custom rules
```

Manage notifications:
```bash
./localhost-magic notify status                          # Show notification config
./localhost-magic notify enable                          # Enable notifications
./localhost-magic notify disable                         # Disable notifications
./localhost-magic notify events service_offline off      # Disable specific event type
./localhost-magic notify events service_discovered on    # Re-enable specific event type
```

### Web Dashboard

Access the dashboard at `http://localhost/` (or any unrecognized hostname).

Features:
- View all services with real-time health status
- Click service names to open them
- Rename services inline
- Toggle "Keep" to persist services when stopped
- Blacklist unwanted services
- Auto-refreshing status indicators

## Testing on Linux (via Orbstack VM)

For testing on a clean Linux environment, use Orbstack or any VM provider.

### 1. Set up Orbstack VM

```bash
# Create a new Ubuntu VM in Orbstack
orb create ubuntu localhost-magic-test

# SSH into the VM
orb ssh localhost-magic-test
```

### 2. Install Go on the VM

```bash
# Download and install Go
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Verify
go version
```

### 3. Build the Project

```bash
# Clone or copy the project to the VM
cd ~
# (copy your project files here)

# Build
go build -o localhost-magic-daemon ./cmd/daemon
go build -o localhost-magic ./cmd/cli
```

### 4. Test the Discovery

Terminal 1 - Start the daemon:
```bash
sudo ./localhost-magic-daemon
```

Terminal 2 - Start a test HTTP server:
```bash
mkdir -p /tmp/myapp
cd /tmp/myapp
python3 -m http.server 8000
```

Terminal 3 - Check discovery:
```bash
./localhost-magic list
# Should show: myapp.localhost -> 127.0.0.1:8000
```

Terminal 4 - Test the proxy:
```bash
curl http://myapp.localhost
# Should show directory listing from Python server
```

### 5. Test Collision Handling

Start another server from the same directory:
```bash
cd /tmp/myapp
python3 -m http.server 8001
```

Check the list:
```bash
./localhost-magic list
# Should show:
# myapp.localhost -> port 8000
# myapp-1.localhost -> port 8001
```

### 6. Test Renaming

```bash
./localhost-magic rename myapp.localhost coolapp.localhost
./localhost-magic list
curl http://coolapp.localhost
```

## Testing on macOS

### 1. Build

```bash
go build -o localhost-magic-daemon ./cmd/daemon
go build -o localhost-magic ./cmd/cli
```

### 2. Start the Daemon

```bash
sudo ./localhost-magic-daemon
```

### 3. Start a Test HTTP Server

Terminal 2:
```bash
mkdir -p /tmp/myapp
cd /tmp/myapp
python3 -m http.server 8000
```

### 4. Check Discovery

Terminal 3:
```bash
./localhost-magic list
# Should show: myapp.localhost -> 127.0.0.1:8000
```

### 5. Test the Proxy

```bash
curl http://myapp.localhost
# Should show directory listing from Python server
```

### Troubleshooting macOS

**Issue**: `lsof` permission denied
- **Solution**: Go to System Settings > Privacy & Security > Full Disk Access and add your terminal application

**Issue**: Port 80 already in use
- **Solution**: macOS may have a service on port 80. Try: `sudo lsof -i :80` to find it

## How It Works

### Port Discovery

**Linux:**
1. Parse `/proc/net/tcp` and `/proc/net/tcp6` for listening sockets
2. Extract socket inode numbers
3. Scan `/proc/<pid>/fd/` to map inodes to PIDs
4. Read `/proc/<pid>/exe` and `/proc/<pid>/cmdline` for process info

**macOS:**
1. Run `lsof -nP -iTCP -sTCP:LISTEN` to get listening ports
2. Parse output to extract PID and port for each socket
3. Use `lsof -p <pid>` to get executable path
4. Use `ps` to get command line arguments

### Name Generation

The tool uses several heuristics to generate the best possible name:

**1. macOS App Bundles**
```
/Applications/Ollama.app/Contents/MacOS/Ollama
        ↓
    "Ollama"
        ↓
   ollama.localhost
```

**2. Script Paths (Node, Python, etc.)**
```
node /home/user/projects/webapp/index.js
        ↓
    "webapp"
        ↓
   webapp.localhost
```

**3. Parent Directory (fallback)**
```
/home/user/projects/myapp/server.js
        ↓
    "myapp"
        ↓
   myapp.localhost
```

**4. Working Directory (for directory-serving tools)**
```
cd ~/projects/myapp && serve
        ↓
    "myapp"  (from CWD)
        ↓
   myapp.localhost
```

Tools that use CWD for naming:
- `serve`, `http-server`, `live-server`
- `python -m http.server`
- `npx` commands

**Collision Handling**: `myapp.localhost` → `myapp-1.localhost` → `myapp-2.localhost`

### Service Health Status

The dashboard shows real-time health indicators:

| Status | Color | Meaning |
|--------|-------|---------|
| Green | #4caf50 | 2xx Success (200, 201, etc.) |
| Orange | #ff9800 | 4xx Client Error (403, 404, etc.) - Normal for root path |
| Red | #f44336 | 5xx Server Error or Offline |
| Gray | #9e9e9e | Service inactive (PID not found) |

### Blacklist

The following services are automatically ignored:

- System binaries (`/usr/sbin/*`, `/usr/bin/*`, `/bin/*`, `/sbin/*`)
- System daemons (`/usr/libexec/*`, `/usr/lib/*`)
- The daemon itself (`localhost-magic-daemon`)

**Exception**: Scripts running through interpreters (Python, Node, etc.) are NOT blacklisted if the script is in a user directory (`/home/*`, `/Users/*`, `/tmp/*`).

### HTTP Detection

Sends a simple HTTP request and verifies the response starts with `HTTP/`.

### Process Identity

Uses SHA256 hash of `realpath(exe) + args` for stable identification across restarts.

## Configuration

Default config location: `~/.config/localhost-magic/services.json`

Example:
```json
[
  {
    "id": "a1b2c3d4...",
    "name": "myapp.localhost",
    "port": 3000,
    "pid": 12345,
    "exe_path": "/usr/bin/node",
    "args": ["/home/user/projects/myapp/server.js"],
    "user_defined": false,
    "is_active": true,
    "keep": false,
    "last_seen": "2026-02-17T20:00:00Z"
  },
  {
    "id": "manual-docker.localhost-172.17.0.2-8080",
    "name": "docker.localhost",
    "port": 8080,
    "target_host": "172.17.0.2",
    "pid": 0,
    "exe_path": "manual",
    "args": [],
    "user_defined": true,
    "is_active": false,
    "keep": true,
    "last_seen": "2026-02-17T20:00:00Z"
  }
]
```

Fields:
- `id`: Unique identifier (SHA256 hash of exe path + args)
- `name`: The .localhost domain name
- `port`: Service port number
- `target_host`: Target IP or hostname (default: `127.0.0.1`, omitted when default)
- `pid`: Process ID (0 if manual entry)
- `exe_path`: Path to executable
- `args`: Command line arguments
- `user_defined`: Whether name was manually set
- `is_active`: Whether service is currently running
- `keep`: Whether to keep in dashboard when stopped
- `last_seen`: Last time service was detected

## Troubleshooting

### Permission Denied on Storage

If you see "permission denied" errors:
```bash
# The daemon runs as root, so storage may be owned by root
sudo chown $USER:$USER ~/.config/localhost-magic/services.json
```

### Services Not Appearing

1. Check if the service is actually listening:
   ```bash
   # Linux
   ss -tlnp | grep LISTEN
   
   # macOS
   lsof -nP -iTCP -sTCP:LISTEN
   ```

2. Verify it's HTTP (not just TCP):
   ```bash
   curl -I http://127.0.0.1:<port>
   ```

3. Check daemon logs:
   ```bash
   sudo ./localhost-magic-daemon 2>&1 | tee daemon.log
   ```

### Wrong Service Names

Clear the storage and restart:
```bash
sudo rm ~/.config/localhost-magic/services.json
sudo ./localhost-magic-daemon
```

### Port 80 Already in Use

Find and stop the process:
```bash
# Linux
sudo lsof -i :80
sudo kill <pid>

# macOS
sudo lsof -i :80
sudo kill <pid>
```

### macOS: lsof Permission Denied

Go to System Settings > Privacy & Security > Full Disk Access and add your terminal application.

### Dashboard Shows Old Services

Services are marked inactive when their PID disappears. They'll be hidden unless "Keep" is enabled. Use the dashboard or CLI to manage keep status:
```bash
./localhost-magic keep myapp.localhost false
```

## Limitations

- **Port 80**: Needs root/sudo to bind privileged port
- **HTTP only**: HTTPS services not yet supported
- **Auto-discovery is local only**: Automatic scanning only finds services on 127.0.0.1 (use `add` with a host for remote targets)
- **macOS**: Uses `lsof` which may require approving terminal in System Settings > Privacy & Security

## API Endpoints

The daemon exposes a REST API on port 80:

- `GET /api/services` - List all services with health status
- `POST /api/rename` - Rename a service (`{"oldName": "...", "newName": "..."}`)
- `POST /api/keep` - Update keep status (`{"name": "...", "keep": true/false}`)
- `POST /api/blacklist` - Add to blacklist (`{"type": "pid|path|pattern", "value": "..."}`)

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the full development roadmap with detailed specs, dependency graphs, and parallelism guide.

### Completed

- [x] macOS support using `lsof`
- [x] Web dashboard for managing services
- [x] Service lifecycle management (keep/persist)
- [x] Health status monitoring
- [x] Manual service entries
- [x] Remote target proxying (Docker, LAN machines)
- [x] Persistent blacklist configuration (1.1)
- [x] Data-driven naming heuristics with JSON rules (1.2)
- [x] Two-tier TLS certificate authority (2.1)
- [x] TLS domain policy with IANA TLD blocklist (2.4)
- [x] Docker container auto-detection (3.1)
- [x] Connection and I/O metrics (5.1)
- [x] systemd/launchd auto-start support (6.1)
- [x] Desktop notifications (6.2)

### Planned (by milestone)

| Milestone | Highlights |
|---|---|
| **1. Core Hardening** | ~~Persistent blacklist~~, ~~naming heuristics~~, subdomain auto-grouping |
| **2. Local Dev TLS** | ~~CA management~~, ~~domain policy~~, trust bootstrap, cert issuer, HTTPS listener, proxy config export ([spec](docs/specs/local-tls-authority.md)) |
| **3. Containers** | ~~Docker auto-detection~~, Compose-aware grouping |
| **4. Peer-to-Peer** | mDNS discovery, access teammates' services as subdomains |
| **5. Observability** | ~~Connection metrics~~, dashboard activity indicators, traffic inspector |
| **6. System Integration** | ~~systemd/launchd auto-start~~, ~~desktop notifications~~, optional GUI |
| **7. Advanced Routing** | Custom TLD support with local DNS resolver |

## License

BSD-3-Clause license
