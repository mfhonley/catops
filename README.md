# CatOps CLI

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Kubernetes-lightgrey.svg)]()

**Fast and convenient server monitoring with AI.** One command to install, zero configuration needed.

Monitor standalone servers or Kubernetes clusters. Get Telegram/Slack/Email alerts and a beautiful web dashboard at [catops.app](https://catops.app).

```bash
# Install in seconds
curl -sfL https://get.catops.app/install.sh | bash
```

> **Looking for Self-Hosted?** Check out [CatOps Self-Hosted](https://catops.app/self-hosted) - single binary with embedded dashboard, no cloud required.

---

## Table of Contents

- [Features](#-features)
- [Quick Start](#-quick-start)
- [Installation](#-installation)
  - [Standalone Servers](#standalone-servers)
  - [Kubernetes Clusters](#kubernetes-clusters)
- [Usage](#-usage)
  - [Basic Commands](#basic-commands)
  - [AI Assistant](#ai-assistant)
  - [Telegram Alerts](#telegram-alerts)
  - [Cloud Mode](#cloud-mode-web-dashboard)
- [Kubernetes Guide](#-kubernetes-guide)
- [Configuration](#-configuration)
- [Log Collection](#-log-collection)
- [Troubleshooting](#-troubleshooting)
- [Contributing](#-contributing)

---

## Features

**Core Monitoring:**
- System metrics (CPU, Memory, Disk, Network, I/O)
- Process monitoring with resource usage
- Service detection (nginx, postgres, redis, docker, and 12+ more)
- **Log collection** from Docker containers
- Real-time Telegram/Slack/Email alerts
- Beautiful web dashboard
- Cross-platform (Linux, macOS, Kubernetes)

**AI-Powered:**
- **AI Assistant** - Ask questions about your server directly from CLI
- Smart alert analysis
- Troubleshooting recommendations

**Deployment Options:**
- **Standalone**: Monitor individual servers (Linux/macOS)
- **Kubernetes**: Monitor entire clusters with DaemonSet

**Alerting:**
- Telegram, Slack, Email notifications
- Alert deduplication (no spam)
- Interactive Telegram buttons (Acknowledge, Silence)
- Instant notifications with detailed context

**Management:**
- Background service with auto-start
- Simple CLI commands
- Automatic updates
- User-level installation (no root required)

---

## Quick Start

### Standalone Server

```bash
# 1. Install
curl -sfL https://get.catops.app/install.sh | bash

# 2. Check status
catops status

# 3. Enable web dashboard (optional)
catops auth login YOUR_TOKEN
```

### Kubernetes Cluster

```bash
# 1. Get your token from https://catops.app/setup

# 2. Install with Helm
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --set auth.token=YOUR_TOKEN

# 3. Verify installation
kubectl get pods -n catops-system
```

That's it! Your servers/nodes appear in the dashboard within 60 seconds.

---

## Installation

### Standalone Servers

**Requirements:**
- Linux (systemd) or macOS (launchd)
- AMD64 or ARM64 architecture
- No root privileges required

**One-Command Install:**

```bash
curl -sfL https://get.catops.app/install.sh | bash
```

**Operating Modes:**
- **Local Mode** (default): Metrics collected locally, view with `catops status`
- **Cloud Mode**: Telegram alerts + Web dashboard at [catops.app](https://catops.app)

**Enable Cloud Mode for Telegram alerts:**
```bash
# 1. Install CatOps
curl -sfL https://get.catops.app/install.sh | bash

# 2. Get token from https://catops.app
catops auth login YOUR_TOKEN

# 3. Configure Telegram bot on catops.app dashboard
```

**From Source (Developers):**

```bash
git clone https://github.com/mfhonley/catops.git
cd catops/cli
go build -o catops ./cmd/catops
./catops --version
```

---

### Kubernetes Clusters

**Requirements:**
- Kubernetes 1.19+
- Helm 3.0+
- metrics-server installed

**Quick Install:**

```bash
# Basic installation (core metrics only)
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --set auth.token=YOUR_TOKEN
```

**With Prometheus (Extended Metrics):**

```bash
# Includes pod labels, owner info, 200+ metrics
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --set auth.token=YOUR_TOKEN \
  --set prometheus.enabled=true \
  --set kubeStateMetrics.enabled=true
```

**Verify Installation:**

```bash
# Check pods are running
kubectl get pods -n catops-system

# Check logs
kubectl logs -n catops-system -l app.kubernetes.io/name=catops --tail=20
```

---

## Usage

### Basic Commands

**Monitoring:**
```bash
catops status              # Show current metrics
catops processes           # Top processes by resource usage
catops restart             # Restart monitoring service
```

**AI Assistant:**
```bash
catops ask "Why is my CPU high?"           # Ask AI about your server
catops ask "What's causing memory spikes?"  # Analyze issues
catops ask "Should I be worried?"           # Get recommendations
```

**Configuration:**
```bash
catops config                       # Show current config
catops set interval=30              # Set metrics collection interval (10-300 seconds)
```

**Service Management:**
```bash
catops service install     # Install as system service (systemd/launchd)
catops service start       # Start service
catops service stop        # Stop service
catops service restart     # Restart service
catops service status      # Check service status
catops service remove      # Remove service
```

**System:**
```bash
catops update              # Check for updates and install
catops uninstall           # Remove CatOps completely
catops cleanup             # Clean up old backup files
catops --version           # Show version
```

### AI Assistant

CatOps includes a **FREE** AI assistant that analyzes your server metrics and provides intelligent answers.

**Features:**
- Context-aware - Analyzes current CPU, Memory, Disk, and top processes
- Fast responses - Optimized for CLI
- Privacy-first - Only sends metrics, not logs
- No subscription required

**Examples:**
```bash
catops ask "Why is my CPU usage high?"
catops ask "What's causing memory spikes?"
catops ask "Should I be worried about disk usage?"
catops ask "Explain what's happening on my server"
```

### Telegram Alerts

**Setup:**
1. Enable Cloud Mode: `catops auth login YOUR_TOKEN`
2. Configure Telegram on [catops.app](https://catops.app) dashboard
3. Connect your Telegram account
4. Start receiving personal alerts

**Interactive Alert Buttons:**

When connected to [catops.app](https://catops.app), alerts include interactive buttons:

```
CPU Spike Detected

server-prod-01
5.2% -> 35.8% (+30.6% spike)

[Acknowledge] [Silence] [Details]
```

- **Acknowledge** - Mark alert as seen, stop re-notifications
- **Silence** - Mute alerts for 30m/1h/2h/24h (useful for maintenance)
- **Details** - Open web dashboard for full metrics and history

### Cloud Mode (Web Dashboard)

**Enable web dashboard at [catops.app](https://catops.app):**

```bash
# 1. Get token from https://catops.app (Profile -> Generate Auth Token)

# 2. Login
catops auth login YOUR_AUTH_TOKEN

# 3. Verify
catops auth info
```

**Benefits:**
- Real-time metrics accessible from anywhere
- Historical data and trends
- Log collection and analysis
- Multi-server monitoring
- Team collaboration
- Mobile-friendly interface

**Operation Modes:**
- **Local Mode** (default): Metrics collected locally, no alerts, works offline
- **Cloud Mode**: Telegram alerts + Web dashboard + Multi-server monitoring + Log collection

### Command Reference

| Command | Description |
|---------|-------------|
| `catops` | Show help and available commands |
| `catops status` | Display current system metrics |
| `catops processes` | Show top processes by resource usage |
| `catops ask "question"` | Ask AI about your server |
| `catops start` | Start monitoring (foreground) |
| `catops restart` | Restart monitoring service |
| `catops config` | Show current configuration |
| `catops set interval=N` | Set collection interval (10-300 sec) |
| `catops auth login TOKEN` | Login with auth token |
| `catops auth logout` | Clear authentication |
| `catops auth info` | Show auth status |
| `catops auth token` | Show full auth token |
| `catops service install` | Install as system service |
| `catops service remove` | Remove system service |
| `catops service start` | Start service |
| `catops service stop` | Stop service |
| `catops service restart` | Restart service |
| `catops service status` | Check service status |
| `catops update` | Update to latest version |
| `catops uninstall` | Remove CatOps completely |
| `catops cleanup` | Clean up old backup files |
| `catops force-cleanup` | Force cleanup stuck processes |
| `catops --version` | Show version |

---

## Kubernetes Guide

### Installation

**Prerequisites Check:**

```bash
# Check Kubernetes version (need 1.19+)
kubectl version --short

# Check Helm installed (need 3.0+)
helm version --short

# Verify metrics-server is working
kubectl top nodes
```

**Install metrics-server (if needed):**

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# For Docker Desktop, allow insecure TLS:
kubectl patch deployment metrics-server -n kube-system \
  --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'
```

**Install CatOps:**

```bash
# Get your auth token from https://catops.app/setup

# Install with Helm
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --set auth.token=YOUR_TOKEN \
  --set prometheus.enabled=true \
  --set kubeStateMetrics.enabled=true
```

### What Gets Monitored

**Node Metrics (per node):**
- CPU, Memory, Disk usage
- Network I/O, HTTPS connections
- Pod count and status
- System information (OS, uptime)

**Pod Metrics (per pod):**
- CPU/Memory usage
- Restart count, container count
- Pod phase (Running/Pending/Failed)
- Namespace, labels, owner info (with Prometheus)

**Cluster Metrics:**
- Total nodes / Ready nodes
- Total pods / Running / Pending / Failed
- Cluster health percentage

### Managing CatOps

**Update:**

```bash
# Update to latest version
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values
```

**Uninstall:**

```bash
# Remove CatOps
helm uninstall catops -n catops-system

# Delete namespace
kubectl delete namespace catops-system
```

---

## Configuration

**Configuration file:** `~/.catops/config.yaml`

**Example:**
```yaml
# Cloud Mode (Set automatically via 'catops auth login')
auth_token: "your_auth_token"
server_id: "507f1f77bcf86cd799439011"

# Monitoring Configuration
collection_interval: 30   # Collect metrics every 30 seconds (default)
```

**Edit configuration:**
```bash
# Via CLI command (recommended)
catops set interval=30

# Or edit file directly
nano ~/.catops/config.yaml
```

### Metrics Collection Interval

**Default:** 30 seconds | **Range:** 10-300 seconds

How often to collect system metrics (CPU, Memory, Disk).

```bash
catops set interval=30    # Default - balanced
catops set interval=15    # More frequent - catches brief spikes
catops set interval=60    # Less frequent - minimal resource usage
```

**When to adjust:**
- **Missing short-lived spikes?** Decrease to 15 seconds
- **Want minimal overhead?** Increase to 60-120 seconds
- **Development environment?** Increase to 60 seconds

---

## Log Collection

CatOps automatically collects logs from various sources when in Cloud Mode:

**Supported Sources:**
- **Docker containers** - Logs from all running containers
- **Docker Compose** - Service logs with container name detection
- **Journald** - System logs on Linux (systemd)
- **Log files** - Common log file locations

**Log Parsing:**
- Automatic format detection (JSON, logfmt, syslog, common log formats)
- Uvicorn/Gunicorn access logs
- Error and warning detection
- HTTP request parsing (method, path, status, duration)
- Stack trace extraction

**View logs in web dashboard at [catops.app](https://catops.app)**

---

## Troubleshooting

### Standalone Issues

**CatOps not starting:**
```bash
# Check status
catops status

# Force cleanup and restart
catops force-cleanup
catops restart

# Check logs (Linux)
journalctl -u catops --since "10 minutes ago"

# Check logs (macOS)
tail -f ~/Library/Logs/catops.log

# Or check default log location
cat /tmp/catops.log
```

**Telegram alerts not working:**
```bash
# Verify Cloud Mode is enabled
catops auth info

# Check Telegram is configured on catops.app dashboard
# Connect your Telegram account in Settings

# Restart daemon
catops restart
```

**Cloud Mode not working:**
```bash
# Check authentication
catops auth info

# Re-login
catops auth logout
catops auth login YOUR_NEW_TOKEN
```

### Kubernetes Issues

**Pods in CrashLoopBackOff:**
```bash
# Check logs for errors
kubectl logs -n catops-system -l app.kubernetes.io/name=catops --tail=100

# Common fix: Install/fix metrics-server
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

**High resource usage:**
```bash
# Check current resource usage
kubectl top pods -n catops-system

# Disable Prometheus to reduce usage
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set prometheus.enabled=false
```

---

## Contributing

We welcome contributions!

**Quick Start:**
```bash
# Fork and clone
git clone https://github.com/YOUR_USERNAME/catops.git
cd catops/cli

# Build
go build -o catops ./cmd/catops

# Test
./catops --version
```

**Areas to contribute:**
- Bug fixes and improvements
- Documentation enhancements
- Platform support (Windows, FreeBSD)
- Feature requests

---

## Support

**Get Help:**
- Telegram: [@mfhonley](https://t.me/mfhonley) - Fastest response
- Email: me@thehonley.org
- Issues: [GitHub Issues](https://github.com/mfhonley/catops/issues)
- Discussions: [GitHub Discussions](https://github.com/mfhonley/catops/discussions)

---

## License

MIT License - see [LICENSE](LICENSE) for details.

**Components:**
- **CatOps CLI**: Open source (MIT)
- **Web Dashboard**: [catops.app](https://catops.app)
- **Backend API**: Cloud infrastructure for metrics storage

---

## Links

- **Website**: [catops.app](https://catops.app)
- **Documentation**: [GitHub](https://github.com/mfhonley/catops)
- **Helm Chart**: [ghcr.io/mfhonley/catops/helm-charts/catops](https://github.com/mfhonley/catops/pkgs/container/catops%2Fhelm-charts%2Fcatops)

---

**Built with love by the open source community**
