# CatOps Development Guide

Guide for developers who want to contribute to CatOps or understand its internal architecture.

---

## Table of Contents

- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Architecture Overview](#architecture-overview)
- [Backend Integration](#backend-integration)
- [Building & Testing](#building--testing)
- [Contributing Guidelines](#contributing-guidelines)

---

## Development Setup

### Prerequisites

- **Go**: 1.21 or higher
- **Git**: For version control
- **Make**: (optional) For build automation
- **Docker**: (optional) For Kubernetes connector development

### Clone and Build

```bash
# Clone repository
git clone https://github.com/mfhonley/catops.git
cd catops

# Build CLI
go build -o catops ./cmd/catops

# Test
./catops --version

# Install locally
sudo cp catops /usr/local/bin/
```

### Development Workflow

```bash
# Make changes to code
vim internal/metrics/collector.go

# Build
go build -o catops ./cmd/catops

# Test locally
./catops status

# Run with debug logging (if implemented)
./catops --debug status
```

---

## Project Structure

```
catops/
├── cmd/
│   └── catops/           # Main CLI entry point
│       └── main.go
│
├── internal/             # Internal packages (not importable)
│   ├── config/          # Configuration management
│   │   ├── config.go
│   │   └── defaults.go
│   │
│   ├── metrics/         # Metrics collection
│   │   ├── collector.go
│   │   ├── cpu.go
│   │   ├── memory.go
│   │   ├── disk.go
│   │   └── network.go
│   │
│   ├── process/         # Process management
│   │   ├── manager.go
│   │   ├── daemon.go
│   │   └── autostart.go
│   │
│   ├── telegram/        # Telegram bot integration
│   │   ├── bot.go
│   │   ├── commands.go
│   │   └── alerts.go
│   │
│   ├── backend/         # Backend API client
│   │   ├── client.go
│   │   ├── events.go
│   │   └── alerts.go
│   │
│   └── ui/              # Terminal UI components
│       └── display.go
│
├── pkg/                 # Public packages (importable)
│   └── utils/
│       └── helpers.go
│
├── config/              # Constants and defaults
│   └── constants.go
│
├── scripts/             # Build and deployment scripts
│   ├── build.sh
│   └── release.sh
│
├── charts/              # Kubernetes Helm charts
│   └── catops/
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
│
├── docs/                # Documentation
│   ├── KUBERNETES_ADVANCED.md
│   ├── TROUBLESHOOTING.md
│   └── DEVELOPMENT.md (this file)
│
├── go.mod               # Go dependencies
├── go.sum
├── README.md            # Main documentation
└── LICENSE              # MIT License
```

---

## Architecture Overview

### System Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    CatOps CLI                           │
│                                                         │
│  ┌───────────────┐                                     │
│  │  cmd/catops   │  Entry point, command routing       │
│  └───────┬───────┘                                     │
│          │                                             │
│  ┌───────▼────────────────────────────────────┐       │
│  │         Internal Packages                   │       │
│  │                                              │       │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  │       │
│  │  │ metrics  │  │ telegram │  │ backend  │  │       │
│  │  │collector │  │   bot    │  │  client  │  │       │
│  │  └──────────┘  └──────────┘  └──────────┘  │       │
│  │                                              │       │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  │       │
│  │  │ process  │  │  config  │  │    ui    │  │       │
│  │  │ manager  │  │ manager  │  │ display  │  │       │
│  │  └──────────┘  └──────────┘  └──────────┘  │       │
│  └──────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────┘
           │                  │                  │
           │ System           │ Telegram         │ HTTPS
           │ APIs             │ API              │
           ▼                  ▼                  ▼
    ┌──────────┐      ┌─────────────┐    ┌──────────────┐
    │  Linux/  │      │  Telegram   │    │   CatOps     │
    │  macOS   │      │   Servers   │    │   Backend    │
    │  System  │      │             │    │ api.catops.io│
    └──────────┘      └─────────────┘    └──────────────┘
```

### Component Responsibilities

**1. Metrics Collector (`internal/metrics/`)**
- Collects system metrics (CPU, memory, disk, network, I/O)
- Cross-platform implementation (Linux/macOS specific code)
- Reads from `/proc`, `sysctl`, system APIs
- Returns structured metric data

**2. Telegram Bot (`internal/telegram/`)**
- Telegram Bot API client
- Command handling (`/status`, `/processes`, `/restart`, etc.)
- Alert sending (threshold violations)
- Group-only mode (security)

**3. Backend Client (`internal/backend/`)**
- HTTP client for CatOps backend API
- Endpoints:
  - `POST /api/data/events` - Service lifecycle events
  - `POST /api/data/alerts` - Alert data
  - `POST /api/downloads/install` - Server registration
- Authentication with `auth_token` and `server_id`

**4. Process Manager (`internal/process/`)**
- Service lifecycle management
- Daemon mode (background service)
- Auto-start management (systemd/launchd)
- Process cleanup and monitoring

**5. Configuration Manager (`internal/config/`)**
- YAML config file management (`~/.catops/config.yaml`)
- Reads/writes configuration
- Mode detection (Local vs Cloud)
- Default values

**6. UI Display (`internal/ui/`)**
- Terminal output formatting
- Color-coded metrics
- Tables and formatting
- Cross-platform terminal compatibility

---

## Backend Integration

### Cloud Mode Architecture

CatOps operates in two modes:

**Local Mode** (default):
- Works completely offline
- Sends alerts to Telegram only
- No backend communication
- Config: `auth_token` and `server_id` are empty

**Cloud Mode**:
- Enabled via `catops auth login <token>`
- Sends metrics to backend
- Available at catops.app dashboard
- Config: `auth_token` and `server_id` are set

### Backend API Endpoints

**1. Server Registration**
```
POST https://api.catops.io/api/downloads/install
Content-Type: application/json

{
  "user_token": "user_provided_token",
  "hostname": "server-name",
  "os": "linux",
  "arch": "amd64"
}

Response:
{
  "user_token": "permanent_token",
  "server_id": "507f1f77bcf86cd799439011"
}
```

**2. Events API** (Service lifecycle)
```
POST https://api.catops.io/api/data/events
Content-Type: application/json
Authorization: Bearer {auth_token}
X-Server-ID: {server_id}

{
  "event_type": "system_monitoring",
  "timestamp": 1640000000,
  "cpu_usage": 45.2,
  "memory_usage": 60.5,
  "disk_usage": 75.0,
  "network": {...},
  "processes": [...]
}
```

**3. Alerts API** (Threshold violations)
```
POST https://api.catops.io/api/data/alerts
Content-Type: application/json
Authorization: Bearer {auth_token}
X-Server-ID: {server_id}

{
  "alert_type": "cpu_high",
  "severity": "warning",
  "value": 95.5,
  "threshold": 85.0,
  "timestamp": 1640000000
}
```

### Data Transmission Flow

```
1. Metrics Collection (every 60s)
   ↓
2. Mode Check (Local or Cloud?)
   ↓
3a. Local Mode:           3b. Cloud Mode:
    - Send to Telegram        - Send to Telegram
    - Skip backend           - Send to Backend API
                              - Store in ClickHouse
                              - Display in Dashboard
```

### Authentication Flow

```
User runs: catops auth login <token>

1. CLI sends token to backend (/api/downloads/install)
2. Backend validates token
3. Backend returns:
   - permanent user_token (stored as auth_token)
   - server_id (MongoDB ObjectId)
4. CLI saves to ~/.catops/config.yaml
5. Cloud Mode activated
6. Metrics start streaming to backend
```

---

## Building & Testing

### Build for Development

```bash
# Quick build
go build -o catops ./cmd/catops

# Build with race detector (Linux/macOS)
go build -race -o catops ./cmd/catops

# Build with debug symbols
go build -gcflags="all=-N -l" -o catops ./cmd/catops
```

### Build for Release

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o catops-linux-amd64 ./cmd/catops

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o catops-linux-arm64 ./cmd/catops

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o catops-darwin-amd64 ./cmd/catops

# macOS ARM64 (M1/M2)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o catops-darwin-arm64 ./cmd/catops
```

### Testing

**Manual Testing:**
```bash
# Build
go build -o catops ./cmd/catops

# Test basic commands
./catops --version
./catops status
./catops processes
./catops config show

# Test Telegram integration
./catops config token=YOUR_TEST_TOKEN
./catops config group=YOUR_TEST_GROUP
./catops restart

# Send test alert
# (manually trigger by setting low thresholds)
./catops set cpu=1 mem=1 disk=1
```

**Note:** Automated tests are planned for future releases. Currently, development relies on manual testing.

### Kubernetes Connector Development

**Build Kubernetes connector:**
```bash
cd kubernetes-connector/
docker build -t catops-k8s:dev .

# Test locally (requires Kubernetes cluster)
kubectl apply -f deploy/test-deployment.yaml
kubectl logs -f deployment/catops-k8s-test
```

---

## Contributing Guidelines

### Before You Start

1. **Check existing issues**: Look for related issues or feature requests
2. **Discuss major changes**: Open an issue to discuss significant changes
3. **Follow code style**: Match existing code formatting and conventions
4. **Test your changes**: Manually test all affected functionality

### Contribution Process

1. **Fork the repository**
   ```bash
   # Fork on GitHub, then clone your fork
   git clone https://github.com/YOUR_USERNAME/catops.git
   cd catops
   ```

2. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/bug-description
   ```

3. **Make your changes**
   - Write clean, readable code
   - Follow Go best practices
   - Add comments for complex logic
   - Update documentation if needed

4. **Test thoroughly**
   ```bash
   # Build
   go build -o catops ./cmd/catops

   # Test all affected commands
   ./catops status
   ./catops processes
   # etc.
   ```

5. **Commit your changes**
   ```bash
   git add .
   git commit -m "feat: add new metric collection for X"
   # or
   git commit -m "fix: resolve issue with Y on macOS"
   ```

   **Commit message format:**
   - `feat:` New feature
   - `fix:` Bug fix
   - `docs:` Documentation changes
   - `refactor:` Code refactoring
   - `perf:` Performance improvement
   - `test:` Adding tests
   - `chore:` Maintenance tasks

6. **Push and create Pull Request**
   ```bash
   git push origin feature/your-feature-name
   ```

   Then create a Pull Request on GitHub with:
   - Clear description of changes
   - Why the change is needed
   - How to test it
   - Screenshots (if UI changes)

### Code Style Guidelines

**Go Code:**
- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Keep functions small and focused
- Add comments for exported functions
- Handle errors explicitly

**Example:**
```go
// GetCPUUsage returns current CPU usage as a percentage.
// Returns error if unable to read CPU stats.
func GetCPUUsage() (float64, error) {
    // Read CPU stats from /proc/stat
    data, err := os.ReadFile("/proc/stat")
    if err != nil {
        return 0, fmt.Errorf("failed to read CPU stats: %w", err)
    }

    // Parse and calculate usage
    usage := parseStats(data)
    return usage, nil
}
```

### Areas for Contribution

**High Priority:**
- 🧪 **Automated testing** - Unit tests, integration tests
- 📝 **Documentation improvements** - Examples, guides, tutorials
- 🐛 **Bug fixes** - Fix reported issues
- 🔍 **Code review** - Review open Pull Requests

**Feature Ideas:**
- 🪟 **Windows support** - Port to Windows platform
- 🐧 **FreeBSD support** - Add FreeBSD compatibility
- 📊 **New metrics** - Additional system metrics
- 🔔 **Alert channels** - Discord, Slack integrations
- 🎨 **UI improvements** - Better terminal formatting

### Getting Help

**Development Questions:**
- 💬 GitHub Discussions: [github.com/mfhonley/catops/discussions](https://github.com/mfhonley/catops/discussions)
- 📧 Email: me@thehonley.org

**Found a Bug?**
- 🐛 GitHub Issues: [github.com/mfhonley/catops/issues](https://github.com/mfhonley/catops/issues)

---

## Release Process

(For maintainers)

**1. Update version:**
- Update version in code
- Update Chart.yaml for Helm chart
- Update CHANGELOG.md

**2. Build binaries:**
```bash
./scripts/build.sh
```

**3. Test release binaries:**
- Test on Linux and macOS
- Verify all commands work
- Test installation script

**4. Create GitHub release:**
- Tag version: `git tag v0.x.x`
- Push tag: `git push origin v0.x.x`
- Create release on GitHub
- Upload binaries

**5. Update Helm chart:**
- Push new chart to ghcr.io
- Update documentation with new version

---

## License

CatOps is licensed under the MIT License. See [LICENSE](../LICENSE) for details.

---

**Questions? Suggestions?**
Open an issue or discussion on GitHub!
