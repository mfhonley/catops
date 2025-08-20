# Moniq CLI

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS-lightgrey.svg)]()

**Moniq CLI** is an ultra-lightweight server monitoring tool that sends real-time alerts and live stats straight to your Telegram group in seconds. One curl command, zero setup hell.

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

### System Management
- **Background Service**: Daemon mode with auto-start capabilities
- **Process Monitoring**: Top processes by resource usage with detailed information
- **Service Control**: Start, stop, restart, and status commands
- **Auto-start Management**: Systemd/launchd service creation and management

### Configuration & Updates
- **Configuration Management**: YAML-based configuration system
- **Update System**: Automatic version checking and updates
- **Operation Modes**: Local (offline) or Cloud (with backend analytics)

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
- **Real-time Monitoring**: Centralized monitoring dashboard
- **Data Analytics**: Historical data and insights
- **Use Case**: Production servers, team monitoring, centralized management

**Mode is automatically determined:**
- **Local**: If `auth_token` or `server_token` is missing
- **Cloud**: If both `auth_token` and `server_token` are present

### Development Requirements (For Building from Source)

- **Go 1.21+**: [Download Go](https://golang.org/dl/)
- **Git**: For cloning the repository
- **Basic Go knowledge**: For building from source

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

**That's it!** The script will automatically:
- Download the correct binary for your platform
- Make it executable
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

**Manual download (if you prefer):**

```bash
# Linux AMD64
curl -L -o moniq https://get.moniq.sh/moniq-linux-amd64

# Linux ARM64  
curl -L -o moniq https://get.moniq.sh/moniq-linux-arm64

# macOS AMD64
curl -L -o moniq https://get.moniq.sh/moniq-darwin-amd64

# macOS ARM64 (Apple Silicon)
curl -L -o moniq https://get.moniq.sh/moniq-darwin-arm64

# Make executable and move to PATH
chmod +x moniq
sudo mv moniq /usr/local/bin/
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

# Make executable and test
chmod +x moniq
./moniq --version
```

### 2. First Run
```bash
# Verify installation
moniq --version

# Start monitoring service
moniq start

# Check system status
moniq status
```

### 3. Basic Usage

```bash
# View system metrics
moniq status

# View top processes
moniq processes

# View configuration
moniq config show
```

## 🔧 Configuration

### Telegram Bot Setup

```bash
# Initialize Telegram configuration
moniq config token=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
moniq config group=-1001234567890

# Set alert thresholds
moniq set cpu=80
moniq set mem=85
moniq set disk=90

# Apply changes
moniq restart
```

### Authentication Setup (Optional)

```bash
# Set your user token for personal dashboard access
moniq config auth=YOUR_AUTH_TOKEN

# Server will be automatically registered and server_token will be saved
# This enables Cloud Mode with backend analytics
```

### Configuration File

The configuration is stored in `~/.moniq/config.yaml`:

```yaml
mode: local                    # automatically determined based on tokens
telegram_token: "BOT_TOKEN"    # Telegram bot token
chat_id: -1001234567890        # Telegram chat ID
auth_token: "USER_TOKEN"       # Your personal dashboard token (optional)
server_token: "SERVER_TOKEN"   # Generated by backend (auto-saved)
cpu_threshold: 80.0            # CPU alert threshold
mem_threshold: 85.0            # Memory alert threshold
disk_threshold: 90.0           # Disk alert threshold
```

**Note:** 
- Configuration is created automatically on first run
- `mode` is automatically determined: `local` (no tokens) or `cloud` (both tokens present)
- `server_token` is automatically generated and saved when you set `auth_token`
- Analytics are only sent to backend when both tokens are present

## 📚 Commands Reference

### Core Commands

| Command | Description | Example |
|---------|-------------|---------|
| `moniq status` | Display current system metrics and alert thresholds | `moniq status` |
| `moniq processes` | Show detailed information about running processes | `moniq processes -n 20` |
| `moniq start` | Start background monitoring service | `moniq start` |
| `moniq restart` | Stop and restart the monitoring service | `moniq restart` |
| `moniq stop` | Stop the monitoring service | `moniq stop` |

### Configuration Commands

| Command | Description | Example |
|---------|-------------|---------|
| `moniq config token=` | Set Telegram bot token | `moniq config token=123:ABC` |
| `moniq config group=` | Set Telegram chat ID | `moniq config group=-100123` |
| `moniq config show` | Display current configuration | `moniq config show` |
| `moniq set` | Configure alert thresholds | `moniq set cpu=90 mem=80` |

### System Commands

| Command | Description | Example |
|---------|-------------|---------|
| `moniq autostart enable` | Enable autostart on boot | `moniq autostart enable` |
| `moniq autostart disable` | Disable autostart on boot | `moniq autostart disable` |
| `moniq autostart status` | Check autostart status | `moniq autostart status` |
| `moniq cleanup` | Clean up old backup files | `moniq cleanup` |
| `moniq update` | Check and install updates | `moniq update` |

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

## 🔍 Troubleshooting

### Common Issues

#### Service Not Starting
```bash
# Check if service is running
moniq status

# Check logs
tail -f /tmp/moniq.log

# Restart service
moniq restart
```

#### Telegram Notifications Not Working
```bash
# Verify configuration
moniq config show

# Check bot token and chat ID
moniq config token=<token>
moniq config group=<chat_id>

# Restart service
moniq restart
```

#### High Resource Usage
```bash
# Check current metrics
moniq status

# View top processes
moniq processes

# Adjust thresholds
moniq set cpu=70 mem=75 disk=85
```

### Log Files
- **Main Log**: `/tmp/moniq.log` - Service and alert logs
- **PID File**: `/tmp/moniq.pid` - Process identification
- **Config**: `~/.moniq/config.yaml` - Configuration file

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

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines for details on how to submit pull requests, report issues, and contribute to the project.

### Contribution Areas
- **New Metrics**: Add additional system monitoring capabilities
- **Platform Support**: Extend support to additional operating systems
- **Integration**: Add support for additional notification services
- **Documentation**: Improve documentation and examples
- **Testing**: Add tests and improve test coverage

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/honley1/moniq.sh/issues)
- **Contributions**: [Pull Requests](https://github.com/honley1/moniq.sh/pulls)



---

**Moniq CLI** - Ultra-lightweight server monitoring. 🚀

Built with ❤️ by the open source community.
