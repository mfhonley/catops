# CatOps

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey.svg)]()

**CatOps** is an ultra-lightweight server monitoring tool that sends real-time alerts and live stats straight to your Telegram group in seconds. One curl command, zero setup hell.

**Simple and flexible** - works offline or with web dashboard at [catops.app](https://catops.app)

```bash
# Install in seconds (from website)
curl -sfL https://get.catops.io/install.sh | bash

# Or from GitHub
git clone https://github.com/mfhonley/catops.git && cd catops && go build -o catops ./cmd/catops
```

## 🚀 Features

### Core Monitoring
- **System Metrics**: CPU, Memory, Disk, Network, I/O monitoring
- **Advanced Metrics**: IOPS, I/O Wait, HTTPS connections, process monitoring
- **Cross-platform Support**: Linux (systemd), macOS (launchd), Windows (Task Scheduler)
- **Kubernetes Support**: Native DaemonSet monitoring for K8s clusters
- **Ultra-Lightweight**: Minimal resource footprint (~15MB binary)
- **Terminal UI**: Clean, color-coded terminal interface

### Alerting & Notifications
- **Telegram Integration**: Instant alerts via Telegram bot with remote commands
- **Configurable Thresholds**: Customizable CPU, Memory, and Disk limits
- **Alert System**: Configurable threshold-based notifications



### System Management
- **Background Service**: Daemon mode with auto-start capabilities
- **Process Monitoring**: Top processes by resource usage with detailed information
- **Service Control**: Start, restart, and status commands
- **Auto-start Management**: Systemd/launchd/Task Scheduler service creation and management
- **Duplicate Process Protection**: Automatic detection and cleanup of multiple instances
- **Zombie Process Cleanup**: Automatic cleanup of defunct processes (Unix only)

### Configuration & Updates
- **Configuration Management**: YAML-based configuration in `~/.catops/config.yaml`
- **Auto Mode Detection**: Automatically switches between Local and Cloud mode
- **Update System**: Automatic version checking and updates



## 📋 Requirements

- **Operating System**: Linux (systemd) or macOS (launchd)
- **Architecture**: AMD64 or ARM64
- **Permissions**: User-level installation (no root required)
- **Network**: Internet access for Telegram bot and web dashboard (optional)

## 🔄 Operation Modes

**Local Mode** (default): Works offline, sends alerts to Telegram only.

**Cloud Mode**: Also sends metrics to web dashboard at [catops.app](https://catops.app) for online monitoring.

*Switch between modes automatically by running `catops auth login <token>`*

## 🛠️ Installation

### Method 1: One-Command Installation (Recommended)

**One curl command, zero setup hell:**

```bash
# Install in one command
curl -sfL https://get.catops.io/install.sh | bash
```

**Quick installation with Telegram setup:**

```bash
# Install with bot token and group ID
curl -sfL https://get.catops.io/install.sh | BOT_TOKEN="your_bot_token" GROUP_ID="your_group_id" sh -
```



**After installation:**
- ✅ **Local Mode**: Working immediately with Telegram bot
- ✅ **Cloud Mode Ready**: Run `catops auth login <token>` to enable web dashboard

**That's it!** The script will automatically:
- Download the correct binary for your platform
- Make it executable
- Configure Telegram bot integration
- Start monitoring service in Local Mode
- Add it to your PATH
- Create configuration directory

**Step-by-step installation with Telegram setup:**

1. **Create a Telegram Bot**
   - Open [@BotFather](https://t.me/botfather) in Telegram
   - Send `/newbot` command
   - Follow instructions to create a bot
   - Save the received token

2. **Create a Group and Add Bot**
   - Create a new group in Telegram
   - Add your bot to the group as administrator
   - Add [@myidbot](https://t.me/myidbot) to the group
   - Send `/getid` in the group and copy the group ID

3. **Install CatOps**
   ```bash
   curl -sfL https://get.catops.io/install.sh | BOT_TOKEN="your_bot_token" GROUP_ID="your_group_id" sh -
   ```
   
   

4. **Verify Installation**
   ```bash
   catops status
   ```

### Method 2: From Source (For Developers & Advanced Users)

**Simple GitHub installation:**

```bash
# Clone and build in one command
git clone https://github.com/mfhonley/catops.git && cd catops && go build -o catops ./cmd/catops

# Make executable and test
chmod +x catops
./catops --version

# Install system-wide (optional)
sudo cp catops /usr/local/bin/
sudo chmod +x /usr/local/bin/catops
```

**Or step by step:**

```bash
# 1. Clone repository
git clone https://github.com/mfhonley/catops.git
cd catops

# 2. Build binary
go build -o catops ./cmd/catops

# 3. Test locally
./catops --version

# 4. Install system-wide (optional)
sudo cp catops /usr/local/bin/
sudo chmod +x /usr/local/bin/catops
```

**Configuration will be created automatically on first run.**

## 🚀 Quick Start

**Get started in seconds with one command:**

### 1. One-Command Installation

**Option A: From Website (Recommended)**
```bash
# Basic installation
curl -sfL https://get.catops.io/install.sh | bash

# Or with Telegram setup
curl -sfL https://get.catops.io/install.sh | BOT_TOKEN="your_bot_token" GROUP_ID="your_group_id" sh -
```

**💡 Pro Tip:** Get ready-to-use commands from [catops.app](https://catops.app)

**Option B: From GitHub**
```bash
# Clone and build
git clone https://github.com/mfhonley/catops.git && cd catops && go build -o catops ./cmd/catops

# Make executable
chmod +x catops

# Test installation
./catops --version
```

### 2. Configure Telegram Bot (Optional)

```bash
# Set bot token
catops config token=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz

# Set group ID
catops config group=-1001234567890

# Show current configuration
catops config show
```

Or let the installer configure it automatically during installation.

**Configuration is stored in `~/.catops/config.yaml`**

### 3. Enable Cloud Mode (Optional but Recommended)

**Get your auth token from [catops.app](https://catops.app):**
1. Visit [catops.app](https://catops.app)
2. Create account or login
3. **Go to Profile**: Click on "Profile" in the left sidebar
4. **Generate Token**: Click the "Generate Auth Token" button
5. **Copy the Token**: Your authentication token will be displayed - copy it to use with CatOps CLI



**Authenticate with backend:**
```bash
# This enables Cloud Mode - your metrics will be available at [catops.app](https://catops.app)
catops auth login your_auth_token

# Verify Cloud Mode is enabled
catops auth info
```

**What happens next:**
- ✅ Server automatically registered with backend
- ✅ Cloud Mode activated
- ✅ Metrics start streaming to dashboard
- ✅ Real-time monitoring available from anywhere



### 4. Start Monitoring

```bash
# Check status
catops status

# View processes
catops processes

# Set alert thresholds
catops set cpu=70 mem=75 disk=85

# Restart monitoring service if needed
catops restart
```

### 5. Enable Autostart (Optional)

```bash
# Enable autostart on boot
catops autostart enable

# Check autostart status
catops autostart status
```

## ☸️ Kubernetes Installation

**Monitor your entire Kubernetes cluster with one Helm command!**

### Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- `metrics-server` installed ([installation guide](https://github.com/kubernetes-sigs/metrics-server#installation))

### Quick Install

```bash
# 1. Get your auth token from https://app.catops.io/settings/integrations

# 2. Install CatOps Kubernetes Connector
helm repo add catops https://charts.catops.io
helm repo update

helm install catops catops/catops \
  --set auth.token=YOUR_AUTH_TOKEN \
  --namespace catops-system \
  --create-namespace
```

**That's it!** 🎉 Your cluster metrics will appear in the dashboard within 1 minute.

### What Gets Monitored?

**Per-Node Metrics:**
- CPU, Memory, Disk usage
- Network I/O
- Running pods count

**Per-Pod Metrics:**
- CPU cores usage
- Memory bytes usage
- Restart count
- Pod phase (Running/Pending/Failed)

**Cluster-Wide Metrics:**
- Total nodes / Ready nodes
- Total pods / Running pods
- Failed/Pending pods

### Advanced Configuration

**Custom resource limits:**
```bash
helm install catops catops/catops \
  --set auth.token=YOUR_TOKEN \
  --set resources.requests.cpu=200m \
  --set resources.requests.memory=256Mi
```

**Run on specific nodes only:**
```bash
helm install catops catops/catops \
  --set auth.token=YOUR_TOKEN \
  --set nodeSelector.workload=monitoring
```

**For full documentation:** See [charts/catops/README.md](charts/catops/README.md)

### Uninstall

```bash
helm uninstall catops -n catops-system
kubectl delete namespace catops-system
```

---

## 📋 Available Commands

### Monitoring Commands

| Command | Description | Example |
|---------|-------------|---------|
| `catops status` | Display current system metrics and alert thresholds | `catops status` |
| `catops processes` | Show detailed information about running processes | `catops processes` |
| `catops restart` | Restart the monitoring service | `catops restart` |
| `catops set` | Configure alert thresholds | `catops set cpu=90 mem=80` |

### Configuration Commands

| Command | Description | Example |
|---------|-------------|---------|
| `catops auth login <token>` | Authenticate with backend for web dashboard access | `catops auth login your_token` |
| `catops auth logout` | Logout and clear authentication | `catops auth logout` |
| `catops auth info` | Show authentication status | `catops auth info` |
| `catops auth token` | Show current authentication token | `catops auth token` |
| `catops config token=` | Set Telegram bot token | `catops config token=123:ABC` |
| `catops config group=` | Set Telegram chat ID | `catops config group=-100123` |
| `catops config show` | Display current configuration | `catops config show` |
| `catops set` | Configure alert thresholds | `catops set cpu=90 mem=80` |

### Authentication & Cloud Mode Commands

#### **`catops auth login <token>`**
**Purpose**: Enables Cloud Mode by authenticating with the backend
**Process**:
1. **First Time**:
   - Registers your server with the backend
   - Receives permanent `user_token` and `server_id` from backend
   - Saves to `~/.catops/config.yaml`: `auth_token` and `server_id`
2. **Subsequent Logins**:
   - If you login with a different token, server ownership is transferred to the new account
3. **Result**: Cloud Mode activated → metrics stream to [catops.app](https://catops.app)

**Example**:
```bash
# Get token from [catops.app](https://catops.app) - go to "Profile" and click "Generate Auth Token"
catops auth login your_generated_auth_token

# Server automatically appears in dashboard
```

#### **`catops auth logout`**
**Purpose**: Disables Cloud Mode by clearing authentication
**Process**:
1. Clears `auth_token` from `~/.catops/config.yaml`
2. Keeps `server_id` (server remains registered in backend)
3. **Result**: Cloud Mode deactivated, metrics no longer sent to backend

**Example**:
```bash
catops auth logout
# Cloud Mode disabled - metrics only available locally
```

#### **`catops auth info`**
**Purpose**: Shows current authentication and Cloud Mode status
**Displays**:
- Authentication status (logged in/logged out)
- Cloud Mode status (enabled/disabled)
- Token information (if authenticated)
- Server registration status

**Example**:
```bash
catops auth info
# Shows authentication and Cloud Mode status
```

### System Commands

| Command | Description | Example |
|---------|-------------|---------|
| `catops autostart enable` | Enable autostart on boot | `catops autostart enable` |
| `catops autostart disable` | Disable autostart on boot | `catops autostart disable` |
| `catops autostart status` | Check autostart status | `catops autostart status` |
| `catops cleanup` | Clean up old backup files and duplicate processes | `catops cleanup` |
| `catops force-cleanup` | Force cleanup of all processes and start fresh | `catops force-cleanup` |
| `catops update` | Check and install updates | `catops update` |
| `catops uninstall` | Completely remove CatOps and all components | `catops uninstall` |

## 📊 Metrics & Monitoring

### System Metrics
- **CPU Usage**: Real-time CPU utilization with core information
- **Memory Usage**: RAM usage with detailed breakdown
- **Disk Usage**: Storage utilization with space information
- **Network**: HTTPS connections and network activity
- **I/O Performance**: IOPS and I/O Wait metrics

### Process Monitoring
- **Top Processes**: CPU and memory usage by process
- **Process Details**: PID, user, command, resource usage
- **Resource Ranking**: Processes sorted by resource consumption
- **Real-time Updates**: Live process information

### Alert System
- **Configurable Thresholds**: Set custom limits for each metric
- **Instant Notifications**: Real-time Telegram alerts
- **Threshold Management**: Easy threshold adjustment via CLI
- **Alert History**: Track system performance over time

### Cloud Mode Data Transmission

#### **What Data is Sent to Backend**
When Cloud Mode is enabled, CatOps automatically sends comprehensive data to [catops.app](https://catops.app):

**Service Lifecycle Events** (via Events API):
- `service_start`: Server startup with system specifications
- `system_monitoring`: Regular metrics every 60 seconds
- `service_stop`: Server shutdown events

**Alert Analytics** (via Alerts API):
- Threshold violations (CPU, Memory, Disk)
- System performance alerts
- Process resource usage data

**Server Specifications**:
- CPU cores count
- Total memory capacity
- Total storage capacity
- Operating system information
- CatOps version

**Real-time Metrics**:
- CPU usage percentage
- Memory usage percentage
- Disk usage percentage
- Network activity (HTTPS requests)
- I/O performance (IOPS, I/O Wait)

**Process Analytics**:
- Top 5 processes by CPU usage
- Top 5 processes by memory usage
- Process summary (total, running, sleeping, zombie)
- Resource consumption ranking

#### **Data Transmission Frequency**
- **Service Events**: Sent immediately (start/stop)
- **System Metrics**: Sent every 60 seconds during monitoring
- **Alert Data**: Sent immediately when thresholds are exceeded
- **Process Data**: Included with every metrics transmission

#### **Data Privacy & Security**
- **User Isolation**: Your data is completely isolated from other users
- **Server Binding**: Each server is tied to your specific account
- **Encrypted Transmission**: All data sent via HTTPS
- **Token Authentication**: Every request includes your unique tokens
- **No Personal Data**: Only system metrics, no personal information

## 🤖 Telegram Bot Integration

### Bot Commands
- `/start` - Start monitoring service
- `/status` - Show current system metrics
- `/processes` - Display top processes
- `/restart` - Restart monitoring service
- `/set` - Set alert thresholds (e.g., `/set cpu=90`)
- `/version` - Show CatOps version
- `/help` - Show available commands

### Setup Instructions
1. Create a Telegram bot via [@BotFather](https://t.me/botfather)
2. Get your bot token
3. Add bot to your group/channel
4. Get your chat ID using [@myidbot](https://t.me/myidbot)
5. Configure: `catops config token=<token> group=<chat_id>`

## 🌐 Web Dashboard (Cloud Mode)

### Access Your Server Metrics Online
When you enable Cloud Mode with `catops auth login <token>`, your server metrics become available at [catops.app](https://catops.app).

### Dashboard Features
- **Real-time Monitoring**: Live metrics streaming from your servers
- **Historical Data**: Track performance trends over time
- **Multi-server View**: Monitor multiple servers from one dashboard
- **Mobile Access**: Responsive design for mobile devices
- **Team Sharing**: Share access with your team members

**Dashboard Overview:**
![Dashboard Overview](docs/images/dashboard-reference.png)

*This screenshot shows the main dashboard interface with real-time metrics, server overview, and monitoring capabilities.*

### How Cloud Mode Works

#### **Automatic Mode Detection**
CatOps automatically determines your operation mode based on `~/.catops/config.yaml`:

- **Local Mode (Default)**: When `auth_token` or `server_id` is missing
- **Cloud Mode**: When both `auth_token` and `server_id` are present

#### **Cloud Mode Activation Process**
1. **Get Auth Token**: Visit [catops.app](https://catops.app), go to "Profile", and click "Generate Auth Token" button
2. **Authenticate**: Run `catops auth login your_auth_token`
3. **Server Registration**: CLI automatically registers your server with the backend
4. **Receive Credentials**: Backend returns permanent `user_token` and `server_id`
5. **Save to Config**: CLI saves both to `~/.catops/config.yaml` as `auth_token` and `server_id`
6. **Mode Switch**: Both values are now present → Cloud Mode activated
7. **Metrics Streaming**: All metrics automatically start streaming to [catops.app](https://catops.app)

#### **What Happens in Cloud Mode**
- **Service Analytics**: Automatically sent to backend API endpoints
- **Real-time Metrics**: CPU, Memory, Disk, Network, I/O data streamed live
- **Process Analytics**: Top processes by resource usage sent to dashboard
- **Alert Analytics**: Threshold violations and system alerts logged
- **Historical Data**: All metrics stored for trend analysis and reporting

#### **Backend API Integration**
Cloud Mode sends data to these secure endpoints:
- **Events API**: `https://api.catops.app/api/data/events` - Service lifecycle events
- **Alerts API**: `https://api.catops.app/api/data/alerts` - Threshold violations
- **Server Management**: `https://api.catops.app/api/downloads/install` - Server registration

#### **Data Security & Privacy**
- **Authentication Required**: All requests include `auth_token` (permanent user_token) and `server_id`
- **Encrypted Transmission**: HTTPS-only communication with backend
- **User Isolation**: Metrics are isolated per user account
- **Server Binding**: Each server tied to specific user account via `server_id`
- **No Data Sharing**: Your data never shared with other users
- **Token Storage**: Credentials stored securely in `~/.catops/config.yaml`

### How to Enable Cloud Mode
1. **Visit [catops.app](https://catops.app)**
2. **Create an account** or login
3. **Go to Profile**: Click on "Profile" in the left sidebar
4. **Generate Token**: Click the "Generate Auth Token" button
5. **Copy the Token**: Your authentication token will be displayed - copy it
6. **Run**: `catops auth login your_auth_token`
7. **Your server will appear** in the dashboard automatically







### 📍 Where to Find Your Auth Token

**Step-by-step guide:**
1. **Login to [catops.app](https://catops.app)**
2. **Go to Profile**: Click "Profile" in the left sidebar
3. **Generate Token**: Click the "Generate Auth Token" button
4. **Copy Token**: Your authentication token will be displayed - click to copy it

**Visual Reference:**
![Auth Token Location](docs/images/auth-token-location.png)

*The screenshot shows the "Generate Auth Token" button in the Profile section. Click this button to generate a new authentication token for your CLI.*





### Local Mode vs Cloud Mode Comparison

| Feature | Local Mode | Cloud Mode |
|---------|------------|------------|
| **Operation** | Completely offline | Backend integration |
| **Data Storage** | Local only | Local + Cloud dashboard |
| **Metrics Access** | CLI + Telegram bot | CLI + Telegram + Web dashboard |
| **Historical Data** | Not available | Full history and trends |
| **Multi-server View** | Not available | Centralized monitoring |
| **Team Access** | Not available | Share with team members |
| **Mobile Monitoring** | Limited (Telegram) | Full mobile dashboard |
| **Resource Usage** | Minimal | Minimal + network calls |
| **Internet Required** | No (except Telegram) | Yes (for dashboard) |
| **Use Case** | Air-gapped servers, testing | Production monitoring, team access |

---

### 📝 Configuration File Structure

**Location**: `~/.catops/config.yaml`

**Example configuration**:

```yaml
# Telegram Bot Settings (Optional - for Telegram alerts)
telegram_token: "1234567890:ABCdefGHIjklMNOpqrsTUVwxyz"
chat_id: -1001234567890

# Cloud Mode Settings (Set automatically via 'catops auth login')
auth_token: "permanent_user_token_from_backend"
server_id: "507f1f77bcf86cd799439011"

# Alert Thresholds (Configurable via 'catops set')
cpu_threshold: 70.0
mem_threshold: 75.0
disk_threshold: 85.0
```

**How it works**:
- **Local Mode**: Only `telegram_token`, `chat_id`, and thresholds are present
- **Cloud Mode**: `auth_token` and `server_id` are added after running `catops auth login`
- **Auto-Detection**: CLI automatically detects mode based on presence of `auth_token` and `server_id`

**Important Notes**:
- ⚠️ `auth_token` is the **permanent user_token** from backend (not the token you generate on dashboard)
- ⚠️ `server_id` is your server's unique MongoDB ObjectId
- ✅ Both values are set automatically during `catops auth login` - no manual editing needed
- ✅ The file is created automatically on first run with default thresholds



## 🔒 Security Features

### Bot Security
- **Group-only Bot**: Bot responds only in configured Telegram groups
- **Unauthorized Access Protection**: Prevents bot usage in other groups
- **Action Logging**: All bot interactions are logged for security monitoring
- **Access Control**: Bot commands work only in authorized groups

### System Security
- **User-level Installation**: No root privileges required
- **Local Configuration**: All settings stored in user's home directory
- **Process Isolation**: Monitoring service runs independently
- **Secure Logging**: All actions logged with timestamps

## 🚀 Advanced Features

### Auto-start Management
```bash
# Enable autostart (creates systemd/launchd/Task Scheduler service)
catops autostart enable

# Check autostart status
catops autostart status

# Disable autostart
catops autostart disable
```

### Process Management
```bash
# View top 10 processes
catops processes

# View top 20 processes
catops processes -n 20

# View top processes by CPU usage
catops processes | grep -A 20 "CPU Usage"
```

### Update Management
```bash
# Check for updates
catops update

# Clean up old backups
catops cleanup
```

### Uninstall Management
```bash
# Completely remove CatOps
catops uninstall
```

## 🏗️ Architecture

### Project Structure
```
catops-cli/
├── cmd/catops/          # Main CLI application
├── internal/           # Internal packages
│   ├── config/        # Configuration management
│   ├── metrics/       # Metrics collection
│   ├── process/       # Process management
│   ├── telegram/      # Telegram bot integration
│   └── ui/           # User interface components
├── pkg/utils/         # Utility functions
├── config/            # Constants and configuration
└── scripts/           # Build and deployment scripts
```

### Key Components
- **Metrics Collector**: Cross-platform system metrics gathering
- **Alert Engine**: Threshold monitoring and notification system
- **Telegram Bot**: Remote monitoring and control interface
- **Process Manager**: Service lifecycle management
- **Configuration System**: YAML-based configuration

### Backend Integration Architecture
- **API Client**: Automatic HTTP requests to backend endpoints
- **Token Management**: Secure storage and validation of auth/server tokens
- **Data Transmission**: Asynchronous metrics streaming to [catops.app](https://catops.app)
- **Mode Detection**: Automatic switching between Local and Cloud modes
- **Server Registration**: Backend API integration for server management
- **Real-time Streaming**: Continuous data transmission during monitoring

## 🧪 Development

**Note**: This project currently uses manual testing and development verification. Automated tests are planned for future releases.

### Building from Source
```bash
# Quick build
git clone https://github.com/mfhonley/catops.git && cd catops && go build -o catops ./cmd/catops

# Or step by step
git clone https://github.com/mfhonley/catops.git
cd catops
go build -o catops ./cmd/catops

# Build completed successfully
```

### Development Requirements
- Go 1.21+
- Linux/macOS development environment
- Telegram bot token for development
- Basic knowledge of system administration

### Development Notes
- **Local Development**: Use `catops auth login <token>` to test Cloud Mode locally
- **Backend Integration**: Monitor API endpoints and data transmission
- **Token Validation**: Verify authentication and server registration flow
- **Mode Switching**: Check automatic switching between Local and Cloud modes
- **Data Transmission**: Monitor metrics streaming to backend APIs

### Adding Screenshots
Save screenshots in `docs/images/` folder and use `![Description](docs/images/filename.png)` syntax.

### Building Binaries
```bash
# Build manually for your platform
go build -o catops ./cmd/catops

# Or build for specific platforms
GOOS=linux GOARCH=amd64 go build -o catops-linux-amd64 ./cmd/catops
GOOS=darwin GOARCH=amd64 go build -o catops-darwin-amd64 ./cmd/catops
GOOS=linux GOARCH=arm64 go build -o catops-linux-arm64 ./cmd/catops
GOOS=darwin GOARCH=arm64 go build -o catops-darwin-arm64 ./cmd/catops
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

**Project Components:**
- **CatOps**: Open source monitoring tool (MIT License)
- **Backend APIs**: Cloud infrastructure for metrics storage
- **Web Dashboard**: [catops.app](https://catops.app) - Centralized monitoring interface
- **Telegram Bot**: Open source bot integration code

## 🤝 Contributing

We welcome contributions! Please see our [contributing guidelines](https://github.com/mfhonley/catops/blob/main/CONTRIBUTING.md) for details.

**Main Areas:**
- **New Features**: Additional monitoring capabilities and platform support
- **Documentation**: Improve guides and examples
- **Testing**: Add automated tests (high priority)
- **Bug Fixes**: Report and fix issues

## 📞 Support & Contact

### Get Help

Having issues? We're here to help!

**Quick Support:**
- 💬 **Telegram**: [@mfhonley](https://t.me/mfhonley) - *Fastest response* ⚡
- 📧 **Email**: me@thehonley.org - *24h response time*

**Community & Development:**
- 🐛 **GitHub Issues**: [Report a bug](https://github.com/mfhonley/catops/issues/new)
- 💬 **GitHub Discussions**: [Community forum](https://github.com/mfhonley/catops/discussions)
- 📚 **Documentation**: [github.com/mfhonley/catops](https://github.com/mfhonley/catops#readme)





---

**CatOps** - Ultra-lightweight server monitoring. 🚀

Built with ❤️ by the open source community.


