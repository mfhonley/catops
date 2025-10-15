# CatOps

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Kubernetes-lightgrey.svg)]()

**Ultra-lightweight server monitoring with real-time alerts.** One command to install, zero configuration needed.

Monitor standalone servers or entire Kubernetes clusters with Telegram alerts and optional web dashboard at [catops.app](https://catops.app).

```bash
# Install in seconds
curl -sfL https://get.catops.io/install.sh | bash
```

---

## Table of Contents

- [Features](#-features)
- [Quick Start](#-quick-start)
- [Installation](#-installation)
  - [Standalone Servers](#standalone-servers)
  - [Kubernetes Clusters](#kubernetes-clusters)
- [Usage](#-usage)
  - [Basic Commands](#basic-commands)
  - [Telegram Bot](#telegram-bot-integration)
  - [Cloud Mode](#cloud-mode-web-dashboard)
- [Kubernetes Guide](#-kubernetes-guide)
- [Configuration](#-configuration)
- [Troubleshooting](#-troubleshooting)
- [Contributing](#-contributing)

---

## 🚀 Features

**Core Monitoring:**
- System metrics (CPU, Memory, Disk, Network, I/O)
- Process monitoring with resource usage
- Real-time Telegram alerts
- Optional web dashboard
- Cross-platform (Linux, macOS, Kubernetes)
- Ultra-lightweight (~15MB binary, ~128MB RAM)

**Deployment Options:**
- **Standalone**: Monitor individual servers (Linux/macOS)
- **Kubernetes**: Monitor entire clusters with DaemonSet

**Alerting:**
- Telegram bot integration with remote commands
- Configurable thresholds (CPU, Memory, Disk)
- Instant notifications

**Management:**
- Background service with auto-start
- Simple CLI commands
- Automatic updates
- User-level installation (no root required)

---

## ⚡ Quick Start

### Standalone Server

```bash
# 1. Install
curl -sfL https://get.catops.io/install.sh | bash

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

## 🛠️ Installation

### Standalone Servers

**Requirements:**
- Linux (systemd) or macOS (launchd)
- AMD64 or ARM64 architecture
- No root privileges required

**One-Command Install:**

```bash
curl -sfL https://get.catops.io/install.sh | bash
```

**With Telegram Bot:**

```bash
curl -sfL https://get.catops.io/install.sh | \
  BOT_TOKEN="your_bot_token" \
  GROUP_ID="your_group_id" \
  bash
```

<details>
<summary>📖 How to get Telegram Bot credentials</summary>

1. **Create Bot:**
   - Open [@BotFather](https://t.me/botfather) in Telegram
   - Send `/newbot` and follow instructions
   - Save the token

2. **Get Group ID:**
   - Create a Telegram group
   - Add your bot as administrator
   - Add [@myidbot](https://t.me/myidbot)
   - Send `/getid` and copy the group ID

</details>

**From Source (Developers):**

```bash
git clone https://github.com/mfhonley/catops.git
cd catops
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

📖 **Detailed Kubernetes guide:** See [Kubernetes Guide](#-kubernetes-guide) below

---

## 📊 Usage

### Basic Commands

**Monitoring:**
```bash
catops status              # Show current metrics
catops processes           # Top processes by resource usage
catops restart             # Restart monitoring service
```

**Configuration:**
```bash
catops config show                              # Show current config
catops config token=YOUR_BOT_TOKEN              # Set Telegram bot token
catops config group=YOUR_GROUP_ID               # Set Telegram group ID
catops set cpu=80 mem=85 disk=90                # Set alert thresholds
```

**Service Management:**
```bash
catops autostart enable    # Enable auto-start on boot
catops autostart disable   # Disable auto-start
catops autostart status    # Check auto-start status
```

**System:**
```bash
catops update              # Check for updates and install
catops uninstall           # Remove CatOps completely
catops --version           # Show version
```

### Telegram Bot Integration

**Available Bot Commands:**
- `/status` - Show system metrics
- `/processes` - Display top processes
- `/restart` - Restart monitoring service
- `/set cpu=90` - Configure alert thresholds
- `/help` - Show all commands

**Setup:**
1. Create bot via [@BotFather](https://t.me/botfather)
2. Add bot to your group as admin
3. Configure CatOps:
   ```bash
   catops config token=YOUR_BOT_TOKEN
   catops config group=YOUR_GROUP_ID
   ```

### Cloud Mode (Web Dashboard)

**Enable web dashboard at [catops.app](https://catops.app):**

```bash
# 1. Get token from https://catops.app (Profile → Generate Auth Token)

# 2. Login
catops auth login YOUR_AUTH_TOKEN

# 3. Verify
catops auth info
```

**Benefits:**
- ✅ Real-time metrics accessible from anywhere
- ✅ Historical data and trends
- ✅ Multi-server monitoring
- ✅ Team collaboration
- ✅ Mobile-friendly interface

**Operation Modes:**
- **Local Mode** (default): Telegram alerts only, works offline
- **Cloud Mode**: Telegram + Web dashboard, requires internet

---

## ☸️ Kubernetes Guide

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

**Stop Monitoring (Temporary):**

```bash
# Option 1: Stop only CatOps connector (Prometheus continues)
kubectl delete daemonset catops -n catops-system

# Option 2: Stop everything including Prometheus
kubectl scale deployment catops-prometheus-server --replicas=0 -n catops-system
kubectl scale deployment catops-kube-state-metrics --replicas=0 -n catops-system
kubectl delete daemonset catops-prometheus-node-exporter -n catops-system
kubectl delete daemonset catops -n catops-system

# Resume later
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values
```

**Reduce Resource Usage:**

```bash
# Disable Prometheus (saves ~500 MB RAM)
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set prometheus.enabled=false

# Reduce collection frequency (saves ~40% CPU)
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set collection.interval=120  # Collect every 2 minutes instead of 1
```

**Update:**

```bash
# Update to latest version
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values

# Update to specific version
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --version 0.2.7 \
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

### Resource Consumption

**Basic Configuration (without Prometheus):**
- Per node: 128-256 MB RAM, 0.1-0.2 CPU
- 3-node cluster: ~300-800 MB RAM total

**With Prometheus:**
- Per node: 128-256 MB RAM, 0.1-0.2 CPU
- Prometheus server: 256-512 MB RAM, 0.1-0.5 CPU
- 3-node cluster: ~1-1.5 GB RAM total

**Recommended for:**
- Basic: Small clusters, Docker Desktop, dev environments
- With Prometheus: Production, full observability, when you need labels/owner info

### Common Kubernetes Issues

**Pods not starting:**
```bash
# Check pod status
kubectl get pods -n catops-system

# Check logs
kubectl logs -n catops-system -l app.kubernetes.io/name=catops --tail=50

# Common fix: Verify metrics-server is working
kubectl top nodes
```

**Metrics not appearing in dashboard:**
```bash
# Verify auth token is correct
kubectl get secret catops -n catops-system -o jsonpath='{.data.auth-token}' | base64 -d

# Check network connectivity
kubectl exec -n catops-system $(kubectl get pod -n catops-system -l app.kubernetes.io/name=catops -o name | head -1) -- \
  wget -O- https://api.catops.io/health
```

📖 **Advanced Kubernetes topics:** See [docs/KUBERNETES_ADVANCED.md](docs/KUBERNETES_ADVANCED.md)

---

## 🔧 Configuration

**Configuration file:** `~/.catops/config.yaml`

**Example:**
```yaml
# Telegram Bot (Optional)
telegram_token: "1234567890:ABC..."
chat_id: -1001234567890

# Cloud Mode (Set automatically via 'catops auth login')
auth_token: "your_auth_token"
server_id: "507f1f77bcf86cd799439011"

# Alert Thresholds
cpu_threshold: 80.0
mem_threshold: 85.0
disk_threshold: 90.0
```

**Edit configuration:**
```bash
# Via CLI commands (recommended)
catops config token=YOUR_BOT_TOKEN
catops config group=YOUR_GROUP_ID
catops set cpu=80 mem=85 disk=90

# Or edit file directly
nano ~/.catops/config.yaml
```

---

## 🔍 Troubleshooting

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
```

**Telegram bot not responding:**
```bash
# Verify configuration
catops config show

# Test bot token is valid
# Send a message to the bot in Telegram

# Reconfigure
catops config token=YOUR_BOT_TOKEN
catops config group=YOUR_GROUP_ID
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

📖 **More troubleshooting:** See [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)

---

## 🤝 Contributing

We welcome contributions!

**Quick Start:**
```bash
# Fork and clone
git clone https://github.com/YOUR_USERNAME/catops.git
cd catops

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

📖 **Development guide:** See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)

---

## 📞 Support

**Get Help:**
- 💬 Telegram: [@mfhonley](https://t.me/mfhonley) - Fastest response
- 📧 Email: me@thehonley.org
- 🐛 Issues: [GitHub Issues](https://github.com/mfhonley/catops/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/mfhonley/catops/discussions)

---

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

**Components:**
- **CatOps CLI**: Open source (MIT)
- **Web Dashboard**: [catops.app](https://catops.app)
- **Backend API**: Cloud infrastructure for metrics storage

---

## 🔗 Links

- **Website**: [catops.app](https://catops.app)
- **Documentation**: [GitHub](https://github.com/mfhonley/catops)
- **Helm Chart**: [ghcr.io/mfhonley/catops/helm-charts/catops](https://github.com/mfhonley/catops/pkgs/container/catops%2Fhelm-charts%2Fcatops)

---

**Built with ❤️ by the open source community**
