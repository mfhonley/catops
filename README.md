# Moniq CLI

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS-lightgrey.svg)]()

**Moniq CLI** is an ultra-lightweight server monitoring tool that sends real-time alerts and live stats straight to your Telegram group in seconds. One curl command, zero setup hell.

**Smart Dual-Mode Operation:**
- **Local Mode**: Works completely offline for air-gapped servers and development
- **Cloud Mode**: Automatic backend integration for centralized monitoring at [dash.moniq.sh](https://dash.moniq.sh)
- **Zero Configuration**: Automatically switches between modes based on your setup

```bash
# Install in seconds (from website)
curl -sfL https://get.moniq.sh/install.sh | bash

# Or from GitHub
git clone https://github.com/honley1/moniq.sh.git && cd moniq.sh && go build -o moniq ./cmd/moniq
```

## 🚀 Features

### Core Monitoring
- **System Metrics**: CPU, Memory, Disk, Network, I/O monitoring
- **Advanced Metrics**: IOPS, I/O Wait, HTTPS connections, process monitoring
- **Cross-platform Support**: Linux (systemd) and macOS (launchd) compatibility
- **Ultra-Lightweight**: Minimal resource footprint (< 1MB binary)
- **Terminal UI**: Clean, color-coded terminal interface

### Alerting & Notifications
- **Telegram Integration**: Instant alerts via Telegram bot with remote commands
- **Configurable Thresholds**: Customizable CPU, Memory, and Disk limits
- **Alert System**: Configurable threshold-based notifications
- **Dual Mode**: Local mode (offline) or Cloud mode (with backend integration)
- **Web Dashboard**: Access metrics online at [dash.moniq.sh](https://dash.moniq.sh) in Cloud Mode

### Cloud Mode & Backend Integration
- **Automatic Mode Detection**: CLI automatically switches between Local and Cloud modes
- **Seamless Authentication**: Simple `moniq auth login <token>` enables Cloud Mode
- **Real-time Metrics Streaming**: Live data automatically sent to [dash.moniq.sh](https://dash.moniq.sh)
- **Server Registration**: Automatic backend registration with hardware specifications
- **Multi-server Management**: Monitor multiple servers from centralized dashboard
- **Historical Data**: Track performance trends and generate reports
- **Team Collaboration**: Share monitoring access with team members
- **Mobile Monitoring**: Full dashboard access from mobile devices

### System Management
- **Background Service**: Daemon mode with auto-start capabilities
- **Process Monitoring**: Top processes by resource usage with detailed information
- **Service Control**: Start, stop, restart, and status commands
- **Auto-start Management**: Systemd/launchd service creation and management
- **Duplicate Process Protection**: Automatic detection and cleanup of multiple instances
- **Zombie Process Cleanup**: Automatic cleanup of defunct processes

### Configuration & Updates
- **Configuration Management**: YAML-based configuration system
- **Update System**: Automatic version checking and updates
- **Operation Modes**: Local (offline) or Cloud (with backend analytics)
- **Web Dashboard**: Cloud Mode provides online access at [dash.moniq.sh](https://dash.moniq.sh)

### Smart Mode Management
- **Automatic Detection**: CLI automatically determines operation mode based on tokens
- **Zero Configuration**: Cloud Mode works out-of-the-box after authentication
- **Seamless Switching**: Easy transition between Local and Cloud modes
- **Token Management**: Secure storage and automatic token validation
- **Backend Integration**: Automatic API endpoint configuration and data transmission

## 📋 Requirements

- **Operating System**: Linux (systemd) or macOS (launchd)
- **Architecture**: AMD64 or ARM64
- **Permissions**: User-level installation (no root required)
- **Network**: Internet access for Telegram bot and backend integration (optional)

## 🔄 Operation Modes

### Local Mode (Default)
- **Offline Operation**: Works completely without internet
- **Local Storage**: All data stored locally
- **No Backend**: No external data transmission
- **Use Case**: Air-gapped servers, development, testing

### Cloud Mode
- **Backend Integration**: Sends metrics and analytics to backend
- **Real-time Monitoring**: Centralized monitoring dashboard at [dash.moniq.sh](https://dash.moniq.sh)
- **Data Analytics**: Historical data and insights
- **Remote Access**: View server metrics from anywhere via web dashboard
- **Use Case**: Production servers, team monitoring, centralized management

### Smart Mode Detection
Moniq CLI automatically determines your operation mode based on configuration tokens:

- **Local Mode**: When `auth_token` or `server_token` is missing
- **Cloud Mode**: When both `auth_token` and `server_token` are present

**No manual configuration required** - the system automatically switches between modes!

### Cloud Mode Features
- **Web Dashboard**: Access your server metrics at [dash.moniq.sh](https://dash.moniq.sh)
- **Real-time Updates**: Live metrics streaming to the dashboard
- **Historical Data**: Track performance over time
- **Team Access**: Share monitoring access with your team
- **Mobile Friendly**: Monitor servers from anywhere
- **Automatic Data Transmission**: Metrics sent automatically every 60 seconds
- **Process Analytics**: Detailed process resource usage data
- **Server Specifications**: Hardware specs automatically collected and sent

## 🛠️ Installation

### Method 1: One-Command Installation (Recommended)

**One curl command, zero setup hell:**

```bash
# Install in one command
curl -sfL https://get.moniq.sh/install.sh | bash
```

**Quick installation with Telegram setup:**

```bash
# Install with bot token and group ID
curl -sfL https://get.moniq.sh/install.sh | BOT_TOKEN="your_bot_token" GROUP_ID="your_group_id" sh -
```

**After installation:**
- ✅ **Local Mode**: Working immediately with Telegram bot
- ✅ **Cloud Mode Ready**: Run `moniq auth login <token>` to enable web dashboard
- ✅ **Automatic Mode Detection**: CLI automatically switches between modes

**That's it!** The script will automatically:
- Download the correct binary for your platform
- Make it executable
- Configure Telegram bot integration
- Start monitoring service in Local Mode
- **Ready for Cloud Mode**: Just run `moniq auth login <token>` to enable web dashboard
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

3. **Install Moniq**
   ```bash
   curl -sfL https://get.moniq.sh/install.sh | BOT_TOKEN="your_bot_token" GROUP_ID="your_group_id" sh -
   ```

4. **Verify Installation**
   ```bash
   moniq status
   ```

### Method 2: From Source (For Developers & Advanced Users)

**Simple GitHub installation:**

```bash
# Clone and build in one command
git clone https://github.com/honley1/moniq.sh.git && cd moniq.sh && go build -o moniq ./cmd/moniq

# Make executable and test
chmod +x moniq
./moniq --version

# Install system-wide (optional)
sudo cp moniq /usr/local/bin/
sudo chmod +x /usr/local/bin/moniq
```

**Or step by step:**

```bash
# 1. Clone repository
git clone https://github.com/honley1/moniq.sh.git
cd moniq.sh

# 2. Build binary
go build -o moniq ./cmd/moniq

# 3. Test locally
./moniq --version

# 4. Install system-wide (optional)
sudo cp moniq /usr/local/bin/
sudo chmod +x /usr/local/bin/moniq
```

**Configuration will be created automatically on first run.**

## 🚀 Quick Start

**Get started in seconds with one command:**

### 1. One-Command Installation

**Option A: From Website (Recommended)**
```bash
# Basic installation
curl -sfL https://get.moniq.sh/install.sh | bash

# Or with Telegram setup
curl -sfL https://get.moniq.sh/install.sh | BOT_TOKEN="your_bot_token" GROUP_ID="your_group_id" sh -
```

**Option B: From GitHub**
```bash
# Clone and build
git clone https://github.com/honley1/moniq.sh.git && cd moniq.sh && go build -o moniq ./cmd/moniq

# Make executable
chmod +x moniq

# Test installation
./moniq --version
```

### 2. Configure Telegram Bot

```bash
# Set bot token
moniq config token=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz

# Set group ID
moniq config group=-1001234567890

# Show current configuration
moniq config show
```

### 3. Enable Cloud Mode (Optional but Recommended)

**Get your auth token from [dash.moniq.sh](https://dash.moniq.sh):**
1. Visit [dash.moniq.sh](https://dash.moniq.sh)
2. Create account or login
3. Copy your personal auth token

**Authenticate with backend:**
```bash
# This enables Cloud Mode - your metrics will be available at [dash.moniq.sh](https://dash.moniq.sh)
moniq auth login your_auth_token

# Verify Cloud Mode is enabled
moniq auth info
```

**What happens next:**
- ✅ Server automatically registered with backend
- ✅ Cloud Mode activated
- ✅ Metrics start streaming to [dash.moniq.sh](https://dash.moniq.sh)
- ✅ Your server appears in the dashboard
- ✅ Real-time monitoring available from anywhere

### 4. Start Monitoring

```bash
# Start monitoring service
moniq start

# Check status
moniq status

# View processes
moniq processes

# Set alert thresholds
moniq set cpu=70 mem=75 disk=85
```

### 5. Enable Autostart (Optional)

```bash
# Enable autostart on boot
moniq autostart enable

# Check autostart status
moniq autostart status
```

## 📋 Available Commands

### Monitoring Commands

| Command | Description | Example |
|---------|-------------|---------|
| `moniq status` | Display current system metrics and alert thresholds | `moniq status` |
| `moniq processes` | Show detailed information about running processes | `moniq processes` |
| `moniq start` | Start background monitoring service | `moniq start` |
| `moniq restart` | Stop and restart the monitoring service | `moniq restart` |

### Configuration Commands

| Command | Description | Example |
|---------|-------------|---------|
| `moniq auth login <token>` | Authenticate with backend for web dashboard access | `moniq auth login your_token` |
| `moniq auth logout` | Logout and clear authentication | `moniq auth logout` |
| `moniq auth info` | Show authentication status | `moniq auth info` |
| `moniq config group=` | Set Telegram chat ID | `moniq config group=-100123` |
| `moniq config show` | Display current configuration | `moniq config show` |
| `moniq set` | Configure alert thresholds | `moniq set cpu=90 mem=80` |

### Authentication & Cloud Mode Commands

#### **`moniq auth login <token>`**
**Purpose**: Enables Cloud Mode by authenticating with the backend
**Process**:
1. **First Time**: Registers your server with the backend and gets a `server_token`
2. **Subsequent**: Transfers server ownership to new auth token
3. **Result**: Both `auth_token` and `server_token` are saved → Cloud Mode activated

**Example**:
```bash
# Get token from [dash.moniq.sh](https://dash.moniq.sh)
moniq auth login eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...

# Server automatically appears in dashboard
```

#### **`moniq auth logout`**
**Purpose**: Disables Cloud Mode by clearing authentication
**Process**:
1. Clears `auth_token` from configuration
2. Keeps `server_token` (server remains registered)
3. **Result**: Cloud Mode deactivated, metrics no longer sent to backend

**Example**:
```bash
moniq auth logout
# Cloud Mode disabled - metrics only available locally
```

#### **`moniq auth info`**
**Purpose**: Shows current authentication and Cloud Mode status
**Displays**:
- Authentication status (logged in/logged out)
- Cloud Mode status (enabled/disabled)
- Token information (if authenticated)
- Server registration status

**Example**:
```bash
moniq auth info
# Shows: ✅ Cloud Mode: ENABLED | 🔐 Authenticated | 🌐 Dashboard: Available
```

### System Commands

| Command | Description | Example |
|---------|-------------|---------|
| `moniq autostart enable` | Enable autostart on boot | `moniq autostart enable` |
| `moniq autostart disable` | Disable autostart on boot | `moniq autostart disable` |
| `moniq autostart status` | Check autostart status | `moniq autostart status` |
| `moniq cleanup` | Clean up old backup files and duplicate processes | `moniq cleanup` |
| `moniq force-cleanup` | Force cleanup of all processes and start fresh | `moniq force-cleanup` |
| `moniq update` | Check and install updates | `moniq update` |
| `moniq uninstall` | Completely remove Moniq CLI and all components | `moniq uninstall` |

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
When Cloud Mode is enabled, Moniq automatically sends comprehensive data to [dash.moniq.sh](https://dash.moniq.sh):

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
- Moniq version

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
- `/version` - Show Moniq CLI version
- `/help` - Show available commands

### Setup Instructions
1. Create a Telegram bot via [@BotFather](https://t.me/botfather)
2. Get your bot token
3. Add bot to your group/channel
4. Get your chat ID
5. Configure CLI: `moniq config token=<token> group=<chat_id>`

## 🌐 Web Dashboard (Cloud Mode)

### Access Your Server Metrics Online
When you enable Cloud Mode with `moniq auth login <token>`, your server metrics become available at [dash.moniq.sh](https://dash.moniq.sh).

### Dashboard Features
- **Real-time Monitoring**: Live metrics streaming from your servers
- **Historical Data**: Track performance trends over time
- **Multi-server View**: Monitor multiple servers from one dashboard
- **Mobile Access**: Responsive design for mobile devices
- **Team Sharing**: Share access with your team members

### How Cloud Mode Works

#### **Automatic Mode Detection**
Moniq CLI automatically determines your operation mode based on configuration:

- **Local Mode (Default)**: When `auth_token` or `server_token` is missing
- **Cloud Mode**: When both `auth_token` and `server_token` are present

#### **Cloud Mode Activation Process**
1. **Get Auth Token**: Visit [dash.moniq.sh](https://dash.moniq.sh) and copy your personal auth token
2. **Authenticate**: Run `moniq auth login your_auth_token`
3. **Server Registration**: CLI automatically registers your server with the backend
4. **Token Exchange**: Backend returns a unique `server_token` for your server
5. **Mode Switch**: Both tokens are now present → Cloud Mode activated
6. **Metrics Streaming**: All metrics automatically start streaming to [dash.moniq.sh](https://dash.moniq.sh)

#### **What Happens in Cloud Mode**
- **Service Analytics**: Automatically sent to backend API endpoints
- **Real-time Metrics**: CPU, Memory, Disk, Network, I/O data streamed live
- **Process Analytics**: Top processes by resource usage sent to dashboard
- **Alert Analytics**: Threshold violations and system alerts logged
- **Historical Data**: All metrics stored for trend analysis and reporting

#### **Backend API Integration**
Cloud Mode sends data to these secure endpoints:
- **Events API**: `https://api.moniq.sh/api/data/events` - Service lifecycle events
- **Alerts API**: `https://api.moniq.sh/api/data/alerts` - Threshold violations
- **Server Management**: `https://api.moniq.sh/api/downloads/install` - Server registration

#### **Data Security & Privacy**
- **Authentication Required**: All requests include `user_token` and `server_token`
- **Encrypted Transmission**: HTTPS-only communication with backend
- **User Isolation**: Metrics are isolated per user account
- **Server Binding**: Each server tied to specific user account
- **No Data Sharing**: Your data never shared with other users

### How to Enable Cloud Mode
1. **Visit [dash.moniq.sh](https://dash.moniq.sh)**
2. **Create an account** or login
3. **Copy your auth token** from the dashboard
4. **Run**: `moniq auth login your_auth_token`
5. **Your server will appear** in the dashboard automatically

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

### Cloud Mode Benefits
- **Centralized Monitoring**: View all servers from one dashboard
- **Historical Insights**: Track performance trends over time
- **Team Collaboration**: Share monitoring access with colleagues
- **Mobile Access**: Monitor servers from anywhere
- **Professional Reporting**: Generate performance reports
- **Alert Management**: Centralized alert configuration
- **Resource Optimization**: Identify performance bottlenecks

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
# Enable autostart (creates systemd/launchd service)
moniq autostart enable

# Check autostart status
moniq autostart status

# Disable autostart
moniq autostart disable
```

### Process Management
```bash
# View top 10 processes
moniq processes

# View top 20 processes
moniq processes -n 20

# View top processes by CPU usage
moniq processes | grep -A 20 "CPU Usage"
```

### Update Management
```bash
# Check for updates
moniq update

# Clean up old backups
moniq cleanup
```

### Uninstall Management
```bash
# Completely remove Moniq CLI
moniq uninstall
```

## 🏗️ Architecture

### Project Structure
```
moniq-cli/
├── cmd/moniq/          # Main CLI application
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
- **Data Transmission**: Asynchronous metrics streaming to [dash.moniq.sh](https://dash.moniq.sh)
- **Mode Detection**: Automatic switching between Local and Cloud modes
- **Server Registration**: Backend API integration for server management
- **Real-time Streaming**: Continuous data transmission during monitoring

## 🧪 Development

### Building from Source
```bash
# Quick build
git clone https://github.com/honley1/moniq.sh.git && cd moniq.sh && go build -o moniq ./cmd/moniq

# Or step by step
git clone https://github.com/honley1/moniq.sh.git
cd moniq.sh
go build -o moniq ./cmd/moniq

# Run tests
go test ./...
```

### Development Requirements
- Go 1.21+
- Linux/macOS development environment
- Telegram bot token for testing
- Basic knowledge of system administration

### Testing Cloud Mode
- **Local Testing**: Use `moniq auth login <test_token>` to test Cloud Mode locally
- **Backend Integration**: Test API endpoints and data transmission
- **Token Validation**: Verify authentication and server registration flow
- **Mode Switching**: Test automatic switching between Local and Cloud modes
- **Data Transmission**: Monitor metrics streaming to backend APIs

### Building Binaries
```bash
# Build manually for your platform
go build -o moniq ./cmd/moniq

# Or build for specific platforms
GOOS=linux GOARCH=amd64 go build -o moniq-linux-amd64 ./cmd/moniq
GOOS=darwin GOARCH=amd64 go build -o moniq-darwin-amd64 ./cmd/moniq
GOOS=linux GOARCH=arm64 go build -o moniq-linux-arm64 ./cmd/moniq
GOOS=darwin GOARCH=arm64 go build -o moniq-darwin-arm64 ./cmd/moniq
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

**Project Components:**
- **Moniq CLI**: Open source monitoring tool (MIT License)
- **Backend APIs**: Cloud infrastructure for metrics storage
- **Web Dashboard**: [dash.moniq.sh](https://dash.moniq.sh) - Centralized monitoring interface
- **Telegram Bot**: Open source bot integration code

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines for details on how to submit pull requests, report issues, and contribute to the project.

### Contribution Areas
- **New Metrics**: Add additional system monitoring capabilities
- **Platform Support**: Extend support to additional operating systems
- **Integration**: Add support for additional notification services
- **Documentation**: Improve documentation and examples
- **Testing**: Add tests and improve test coverage

### Cloud Mode Enhancements
- **Additional APIs**: Extend backend integration capabilities
- **Data Formats**: Improve metrics data structure and transmission
- **Security Features**: Enhance token validation and encryption
- **Dashboard Integration**: Add new dashboard features and widgets
- **Performance Optimization**: Optimize data transmission and storage

## 📞 Support & Contact

### Get Help
- **Email**: honley@moniq.sh
- **Telegram**: [@moniqsh](https://t.me/moniqsh)
- **GitHub Issues**: [Report Issues](https://github.com/honley1/moniq.sh/issues)
- **GitHub Discussions**: [Community Forum](https://github.com/honley1/moniq.sh/discussions)

### Cloud Mode Support
- **Dashboard Access**: [dash.moniq.sh](https://dash.moniq.sh) - Get your auth token
- **Authentication Issues**: Contact support for token problems
- **Data Transmission**: Check logs for backend communication issues
- **Mode Switching**: Verify token configuration for Local/Cloud mode
- **API Integration**: Report backend API issues or improvements

### Community
- **Contributions**: [Pull Requests](https://github.com/honley1/moniq.sh/pulls)
- **Feature Requests**: [GitHub Issues](https://github.com/honley1/moniq.sh/issues)
- **Bug Reports**: [GitHub Issues](https://github.com/honley1/moniq.sh/issues)

### Cloud Mode Development
- **Backend Integration**: Help improve API endpoints and data transmission
- **Dashboard Features**: Contribute to [dash.moniq.sh](https://dash.moniq.sh) development
- **Security Enhancements**: Improve token validation and encryption
- **Performance Optimization**: Optimize metrics collection and transmission
- **Testing & QA**: Test Cloud Mode functionality across different platforms

---

**Moniq CLI** - Ultra-lightweight server monitoring. 🚀

Built with ❤️ by the open source community.

**Need help?** Contact us at [honley@moniq.sh](mailto:honley@moniq.sh) or [@moniqsh](https://t.me/moniqsh) on Telegram.
