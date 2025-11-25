# CatOps

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Kubernetes-lightgrey.svg)]()

**Ultra-lightweight server monitoring with real-time alerts.** One command to install, zero configuration needed.

Monitor standalone servers or entire Kubernetes clusters with Telegram alerts and optional web dashboard at [catops.app](https://catops.app).

```bash
# Install in seconds
curl -sfL https://get.catops.app/install.sh | bash
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
- Intelligent spike detection (sudden spikes, gradual rises, anomalies)
- Alert deduplication (no spam)
- Interactive Telegram buttons (Acknowledge, Silence)
- Instant notifications with detailed context

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

## 🛠️ Installation

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
catops set cpu=85 mem=90 disk=95                # Set alert thresholds
catops set spike=30 gradual=15                  # Set spike detection sensitivity
catops set anomaly=4.0                          # Set anomaly detection (std deviations)
catops set renotify=120                         # Set re-notification interval (minutes)
catops set interval=30 buffer=40 resolution=10  # Advanced monitoring config
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

### Telegram Alerts (Cloud Mode)

**Setup:**
1. Enable Cloud Mode: `catops auth login YOUR_TOKEN`
2. Configure Telegram on [catops.app](https://catops.app) dashboard
3. Connect your Telegram account
4. Start receiving personal alerts

**Interactive Alert Buttons:**

When connected to [catops.app](https://catops.app), alerts include interactive buttons:

```
🔴 CPU Spike Detected

server-prod-01
📈 5.2% → 35.8% (+30.6% spike)

[Acknowledge] [Silence ▼] [Details]
```

- **Acknowledge** - Mark alert as seen, stop re-notifications
- **Silence** - Mute alerts for 30m/1h/2h/24h (useful for maintenance)
- **Details** - Open web dashboard for full metrics and history

**Alert Types:**
- **Threshold** - Metric exceeded configured limit (CPU > 85%)
- **Sudden Spike** - Rapid increase detected (5% → 40% in 15 seconds)
- **Gradual Rise** - Sustained increase over time (15% → 32% over 5 minutes)
- **Anomaly** - Statistical outlier (configurable, default: 4.0σ from average)
  - Uses standard deviation to detect unusual values
  - Adjust with `catops set anomaly=5.0` for less sensitivity
- **Recovery** - Alert resolved, metrics back to normal

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
- **Local Mode** (default): Metrics collected locally, no alerts, works offline
- **Cloud Mode**: Telegram alerts + Web dashboard + Multi-server monitoring

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
  wget -O- https://api.catops.app/health
```

📖 **Advanced Kubernetes topics:** See [docs/KUBERNETES_ADVANCED.md](docs/KUBERNETES_ADVANCED.md)

---

## 🔧 Configuration

**Configuration file:** `~/.catops/config.yaml`

**Example:**
```yaml
# Cloud Mode (Set automatically via 'catops auth login')
auth_token: "your_auth_token"
server_id: "507f1f77bcf86cd799439011"

# Alert Thresholds
cpu_threshold: 85.0
mem_threshold: 90.0
disk_threshold: 95.0

# Alert Sensitivity (Advanced)
sudden_spike_threshold: 30.0      # Alert on CPU/Memory changes > 30%
gradual_rise_threshold: 15.0      # Alert on sustained increases > 15%
anomaly_threshold: 4.0            # Alert on statistical anomalies > 4σ (std deviations)
alert_renotify_interval: 120      # Re-notify every 2 hours (minutes)
alert_deduplication: true         # Enable alert deduplication (prevents spam, recommended: true)

# Monitoring Configuration (Advanced)
collection_interval: 15           # Collect metrics every 15 seconds
buffer_size: 20                   # Store 20 data points for statistics
alert_resolution_timeout: 5       # Mark alerts resolved after 5 minutes
```

**Edit configuration:**
```bash
# Via CLI commands (recommended)
catops set cpu=85 mem=90 disk=95

# Configure alert sensitivity
catops set spike=30 gradual=15 anomaly=4.0 renotify=120

# Configure monitoring (advanced)
catops set interval=30 buffer=40 resolution=10

# Or edit file directly
nano ~/.catops/config.yaml
```

### Alert Sensitivity Settings

Control how often you receive alerts:

**For production servers (default, balanced):**
```bash
catops set cpu=85 spike=30 gradual=15 anomaly=4.0 renotify=120
catops restart
```

**For critical servers (more sensitive):**
```bash
catops set cpu=75 spike=25 gradual=12 anomaly=3.0 renotify=60
catops restart
```

### Understanding Alert Sensitivity Parameters

#### 1. **`spike` - Sudden Spike Threshold**
**Default:** 30% | **Range:** 0-100%

Detects rapid changes in CPU/Memory usage between consecutive measurements.

**How it works:**
- Compares current metric value vs the immediately previous measurement
- Uses single point-to-point percentage change across one collection interval
- Example: CPU jumps from 5% to 35% in one interval (15s) = 30% spike
- Calculation: `((current - previous) / previous) × 100`

**When to adjust:**
- **Too many spike alerts?** Increase to 35-40%
- **Missing sudden issues?** Decrease to 20-25%

**Real-world scenarios:**
- `spike=25%`: More sensitive, detects most sudden problems
- `spike=30%`: Production default (balanced)
- `spike=40%`: Dev/staging (only extreme cases)

---

#### 2. **`gradual` - Gradual Rise Threshold**
**Default:** 15% | **Range:** 0-100%

Detects sustained increases over a fixed 5-minute analysis window.

**How it works:**
- Compares oldest value in 5-minute window vs current value
- Window duration is **FIXED at 5 minutes** (not affected by buffer/interval settings)
- Triggers if percentage change exceeds threshold
- Example: CPU at 15% five minutes ago, now at 30% = 15% gradual rise
- Note: Algorithm compares only start vs end; the path between doesn't affect detection

**When to adjust:**
- **Too many gradual alerts?** Increase to 18-20%
- **Want early warning?** Decrease to 10-12%

**Real-world scenarios:**
- `gradual=10%`: More sensitive, catch slow memory leaks early
- `gradual=15%`: Production default (balanced)
- `gradual=20%`: Servers with expected load variations

---

#### 3. **`anomaly` - Statistical Anomaly Threshold**
**Default:** 4.0σ | **Range:** 1.0-10.0 standard deviations

Detects values that are statistically unusual compared to historical average.

**How it works:**
- Calculates average (μ) and standard deviation (σ) over 5 minutes
- Measures how many σ current value deviates from average
- Triggers if deviation exceeds threshold

**Mathematical explanation:**
```
Current value: 25%
Historical avg (μ): 8%
Std deviation (σ): 4%

Deviation = |25 - 8| / 4 = 4.25σ
If anomaly=4.0 → Alert triggered (4.25 > 4.0)
If anomaly=5.0 → No alert (4.25 < 5.0)
```

**When to adjust:**
- **Too many anomaly alerts for small changes?** Increase to 4.5-5.0σ
- **Missing unusual patterns?** Decrease to 2.5-3.0σ
- **Getting alerts like "13.2% (4.5σ)"?** Your threshold is 4.0σ, increase to 5.0σ

**Statistical context:**
- **1σ**: ~68% of values fall within ±1σ (very common)
- **2σ**: ~95% of values fall within ±2σ (common)
- **3σ**: ~99.7% of values fall within ±3σ (rare)
- **4σ**: ~99.99% of values fall within ±4σ (very rare, default)
- **5σ**: Extreme outlier (once in 1.7 million events)

**Real-world scenarios:**
- `anomaly=3.0σ`: Critical servers (catch more unusual patterns)
- `anomaly=4.0σ`: Production default (balanced)
- `anomaly=5.0σ`: Less sensitive (reduce noise further)
- `anomaly=6.0σ`: Dev/staging (only extreme anomalies)

---

#### 4. **`renotify` - Re-notification Interval**
**Default:** 120 minutes | **Range:** Any positive integer

How often to resend alert if problem persists.

**How it works:**
- After initial alert, wait N minutes before sending same alert again
- Only applies to unacknowledged, active alerts
- Stops if alert is acknowledged or resolved

**When to adjust:**
- **Alert fatigue?** Increase to 180-240 minutes
- **Need frequent reminders?** Decrease to 60 minutes
- **Critical systems?** Decrease to 30-60 minutes

**Real-world scenarios:**
- `renotify=30`: Critical infrastructure (frequent reminders)
- `renotify=60`: More frequent notifications
- `renotify=120`: Production default (balanced)
- `renotify=240`: Dev/staging (occasional reminders)

---

### Advanced Monitoring Configuration

#### 5. **`interval` - Metrics Collection Interval**
**Default:** 15 seconds | **Range:** 10-300 seconds

How often to collect system metrics (CPU, Memory, Disk).

**How it works:**
- Collects metrics every N seconds
- Lower interval = more frequent measurements, catches short-lived spikes
- Higher interval = less CPU overhead, may miss brief spikes
- **Note:** Gradual rise window is FIXED at 5 minutes regardless of interval

**Impact on detection:**
```
interval=15s → Checks every 15 seconds (more sensitive to brief spikes)
interval=30s → Checks every 30 seconds (half the overhead)
interval=60s → Checks every 60 seconds (minimal overhead, may miss brief spikes)

Note: Gradual rise detection always uses 5-minute window
```

**When to adjust:**
- **CatOps using too much CPU?** Increase to 30-60 seconds
- **Missing short-lived spikes?** Keep at 15 seconds
- **Development environment?** Increase to 60 seconds
- **Critical production?** Keep at 15 seconds for accuracy

**Real-world scenarios:**
- `interval=15`: Production default (accurate spike detection)
- `interval=30`: Low-resource servers (half the overhead)
- `interval=60`: Dev/staging (minimal overhead)
- `interval=120`: Very lightweight monitoring (background only)

---

#### 6. **`buffer` - Historical Data Buffer Size**
**Default:** 20 data points | **Range:** 10-100 data points

How many historical metric values to store for statistical analysis and anomaly detection.

**How it works:**
- Stores last N metric readings in memory
- Used for calculating averages, standard deviation for anomaly detection
- Provides historical context for status display and trend analysis
- **Note:** Does NOT affect gradual rise window (fixed at 5 minutes)

**Buffer size guidelines:**
- Buffer should hold at least 5 minutes of data: `buffer_size ≥ 300 / interval`
- Examples:
  - interval=15s → buffer≥20 points (covers 5 minutes)
  - interval=30s → buffer≥10 points (covers 5 minutes)
  - interval=60s → buffer≥5 points (covers 5 minutes)

**When to adjust:**
- **Memory-constrained systems?** Decrease to 10-15 points
- **Need longer trend analysis?** Increase to 40-60 points
- **Servers with stable load?** Keep at 20 points
- **Highly variable workloads?** Increase to 30-40 points

**Real-world scenarios:**
- `buffer=10`: Minimal memory (sufficient for 30s intervals, marginal for 15s)
- `buffer=20`: Production default (holds 5 minutes at 15s interval)
- `buffer=40`: Better statistics (more data points for anomaly detection)
- `buffer=60`: Best statistics (maximum historical data for trend analysis)

**Memory usage:**
Each data point uses ~24 bytes. Per metric type (CPU/Memory/Disk):
- `buffer=10` → ~240 bytes × 3 metrics = ~720 bytes
- `buffer=20` → ~480 bytes × 3 metrics = ~1.4 KB
- `buffer=60` → ~1.4 KB × 3 metrics = ~4.2 KB

**Important:** Buffer size does NOT change detection windows (always 5 minutes). It only affects:
1. Statistical accuracy - more data points = better anomaly detection
2. Minimum required to hold 5-minute window data for gradual rise detection
3. If buffer is too small for your interval, gradual rise may not work properly

---

#### 7. **`resolution` - Alert Resolution Timeout**
**Default:** 5 minutes | **Range:** 1-60 minutes

How long to wait before marking an alert as resolved after metrics return to normal.

**How it works:**
- When metric drops below threshold, starts timer
- If metric stays below threshold for N minutes → sends "RESOLVED" notification
- If metric spikes again within N minutes → doesn't send resolved (prevents spam)
- Prevents flapping alerts (on/off/on/off)

**Example timeline:**
```
Time 0:00 - CPU = 95% → Alert triggered
Time 0:05 - CPU drops to 70%
Time 0:10 - Still at 70% for 5 minutes → "RESOLVED" sent (with resolution=5)
```

**Flapping prevention:**
```
Time 0:00 - CPU = 95% → Alert triggered
Time 0:02 - CPU drops to 70%
Time 0:04 - CPU spikes to 90% again
→ No "RESOLVED" sent, alert continues (prevents spam)
```

**When to adjust:**
- **Flapping alerts (on/off frequently)?** Increase to 10-15 minutes
- **Need immediate resolution notifications?** Decrease to 2-3 minutes
- **Stable servers?** Keep at 5 minutes
- **Unstable workloads?** Increase to 10 minutes

**Real-world scenarios:**
- `resolution=2`: Fast resolution notifications (may cause flapping)
- `resolution=5`: Production default (balanced)
- `resolution=10`: High-variability workloads (prevents flapping)
- `resolution=15`: Very unstable systems (maximum stability)

---

### Quick Reference Table

**Alert Sensitivity:**

| Scenario | spike | gradual | anomaly | renotify |
|----------|-------|---------|---------|----------|
| **Critical production** | 25% | 12% | 3.0σ | 60min |
| **Standard production (default)** | 30% | 15% | 4.0σ | 120min |
| **Dev/Staging** | 40% | 20% | 5.0σ | 240min |
| **High traffic (expected spikes)** | 35% | 18% | 4.5σ | 120min |
| **Low traffic (stable)** | 28% | 13% | 3.5σ | 90min |

**Monitoring Configuration:**

| Scenario | interval | buffer | resolution |
|----------|----------|--------|------------|
| **Critical production** | 15s | 20 | 5min |
| **Standard production (default)** | 15s | 20 | 5min |
| **Low-resource servers** | 30s | 20 | 10min |
| **Dev/Staging** | 60s | 15 | 10min |
| **Extended analysis** | 15s | 40 | 5min |
| **Minimal overhead** | 120s | 10 | 15min |

---

### How Alert Types Interact

**Priority order (highest to lowest):**
1. **Sudden Spike** - Immediate danger (memory leak, attack)
2. **Gradual Rise** - Growing problem (slow leak, increasing load)
3. **Anomaly** - Unusual pattern (statistical outlier)
4. **Threshold** - Limit exceeded (only sent if NO spikes/anomalies)

**Why threshold alerts might not appear:**
- If CPU constantly fluctuates by small amounts, anomaly alerts fire
- Increase `anomaly` threshold to reduce sensitivity
- This allows threshold alerts to be sent when CPU > configured limit

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

**Too many alerts (alert spam):**
```bash
# Note: Defaults are already balanced (spike=30, gradual=15, anomaly=4.0, renotify=120)
# If still getting too many alerts, increase thresholds:

# For even less noise (dev/staging servers)
catops set spike=40 gradual=20 anomaly=5.0 renotify=240
catops restart

# If getting too many anomaly alerts for small changes:
catops set anomaly=5.0  # Increase anomaly threshold
catops restart
```

**Not receiving threshold alerts:**
```bash
# Threshold alerts only send when there are NO spikes/anomalies
# Defaults are already configured for production use
# To allow more threshold alerts, increase spike detection thresholds:
catops set spike=35 gradual=18 anomaly=5.0
catops restart
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
