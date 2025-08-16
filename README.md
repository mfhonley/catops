# Moniq CLI

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS-lightgrey.svg)]()

**Moniq CLI** is a lightweight, open-source system monitoring tool designed for server administrators and DevOps engineers. It provides system metrics, alerting, and integration capabilities with both local and cloud operation modes.

## 🚀 Features

### Core Monitoring
- **System Metrics**: CPU, Memory, Disk, Network, I/O monitoring
- **Advanced Metrics**: IOPS, I/O Wait, HTTPS connections, process monitoring
- **Cross-platform Support**: Linux (systemd) and macOS (launchd) compatibility
- **Lightweight**: Minimal resource footprint (< 1MB binary)
- **Terminal UI**: Clean, color-coded terminal interface

### Alerting & Notifications
- **Telegram Integration**: Instant alerts via Telegram bot with remote commands
- **Configurable Thresholds**: Customizable CPU, Memory, and Disk limits
- **Alert System**: Configurable threshold-based notifications
- **Dual Mode Operation**: Local mode (no backend) or Cloud mode (with analytics)

### System Management
- **Background Service**: Daemon mode with auto-start capabilities
- **Process Monitoring**: Top processes by resource usage with detailed information
- **Service Control**: Start, stop, restart, and status commands
- **Auto-start Management**: Systemd/launchd service creation and management

### Configuration & Updates
- **Configuration Management**: YAML-based configuration system
- **Update System**: Automatic version checking and updates
- **Local/Cloud Modes**: Flexible operation modes

## 📋 Requirements

- **Operating System**: Linux (systemd) or macOS (launchd)
- **Go Version**: 1.21 or higher (for building from source)
- **Architecture**: AMD64 or ARM64
- **Permissions**: User-level installation (no root required)
- **Network**: Internet access for Telegram bot and cloud mode features

## 🛠️ Installation

### From Source (Recommended)

```bash
# Clone the repository
git clone https://github.com/your-org/moniq-cli.git
cd moniq-cli

# Build the binary
go build -o moniq ./cmd/moniq

# Install to system
sudo cp moniq /usr/local/bin/
sudo chmod +x /usr/local/bin/moniq
```

### Quick Start

```bash
# Start monitoring service
moniq start

# Check system status
moniq status

# View top processes
moniq processes

# Configure Telegram bot
moniq config token=YOUR_BOT_TOKEN
moniq config group=YOUR_CHAT_ID

# Set alert thresholds
moniq set cpu=80 mem=85 disk=90

# Apply changes
moniq restart
```

## 🔧 Configuration

### Basic Setup

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

### Configuration File

The configuration is stored in `~/.moniq/config.yaml`:

```yaml
mode: local                    # local or cloud
telegram_token: "BOT_TOKEN"    # Telegram bot token
chat_id: -1001234567890        # Telegram chat ID
auth_token: ""                 # Backend auth token (cloud mode)
server_token: ""               # Backend server token (cloud mode)
cpu_threshold: 80.0            # CPU alert threshold
mem_threshold: 85.0            # Memory alert threshold
disk_threshold: 90.0           # Disk alert threshold
```

## 📚 Commands Reference

### Core Commands

| Command | Description | Example |
|---------|-------------|---------|
| `moniq status` | Display current system metrics and alert thresholds | `moniq status` |
| `moniq processes` | Show detailed information about running processes | `moniq processes -n 20` |
| `moniq start` | Start background monitoring service | `moniq start` |
| `moniq restart` | Stop and restart the monitoring service | `moniq restart` |

### Configuration Commands

| Command | Description | Example |
|---------|-------------|---------|
| `moniq config token=` | Set Telegram bot token | `moniq config token=123:ABC` |
| `moniq config group=` | Set Telegram chat ID | `moniq config group=-100123` |
| `moniq config show` | Display current configuration | `moniq config show` |
| `moniq set` | Configure alert thresholds | `moniq set cpu=90 mem=80` |

### Authentication Commands

| Command | Description | Example |
|---------|-------------|---------|
| `moniq auth login` | Login with backend authentication token | `moniq auth login <token>` |
| `moniq auth logout` | Logout and clear authentication | `moniq auth logout` |
| `moniq auth info` | Show authentication status | `moniq auth info` |

### System Commands

| Command | Description | Example |
|---------|-------------|---------|
| `moniq autostart enable` | Enable autostart on boot | `moniq autostart enable` |
| `moniq autostart disable` | Disable autostart on boot | `moniq autostart disable` |
| `moniq autostart status` | Check autostart status | `moniq autostart status` |
| `moniq cleanup` | Clean up old backup files | `moniq cleanup` |
| `moniq update` | Check and install updates | `moniq update` |

## 🔄 Operation Modes

### Local Mode (Default)
- **No Backend Dependency**: Works completely offline
- **Telegram Alerts**: Local notifications via Telegram bot
- **Local Configuration**: All settings stored locally
- **Process Monitoring**: Full system monitoring capabilities
- **No Analytics**: Metrics are not sent to external services

### Cloud Mode
- **Backend Integration**: Full backend analytics and management
- **Server Registration**: Automatic server registration and management
- **Analytics**: Metrics and alerts sent to backend dashboard
- **Version Updates**: Automatic update checking and notifications
- **Multi-server Management**: Centralized server management

### Switching Between Modes

```bash
# Switch to Cloud Mode
moniq auth login <your_auth_token>

# Switch to Local Mode
moniq auth logout
```

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
- **Configuration System**: YAML-based configuration with mode support

## 🧪 Development

### Building from Source
```bash
# Clone repository
git clone https://github.com/your-org/moniq-cli.git
cd moniq-cli

# Install dependencies
go mod download

# Build binary
go build -o moniq ./cmd/moniq

# Run tests
go test ./...
```

### Development Requirements
- Go 1.21+
- Linux/macOS development environment
- Telegram bot token for testing
- Basic knowledge of system administration

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

- **Issues**: [GitHub Issues](https://github.com/your-org/moniq-cli/issues)
- **Contributions**: [Pull Requests](https://github.com/your-org/moniq-cli/pulls)

## 🔮 Roadmap

### Upcoming Features
- [ ] **Performance Optimization**: Enhanced resource efficiency
- [ ] **Additional Metrics**: Extended system monitoring capabilities
- [ ] **Platform Support**: Additional operating system support

### Version History
- **v0.1.6** - Current stable release
- **v0.1.5** - Added I/O metrics and process monitoring
- **v0.1.4** - Improved Telegram bot integration
- **v0.1.3** - Added cloud mode and backend integration
- **v0.1.2** - Enhanced alert system and thresholds
- **v0.1.1** - Basic monitoring and Telegram alerts
- **v0.1.0** - Initial release with core functionality

---

**Moniq CLI** - Open-source system monitoring made simple. 🚀
