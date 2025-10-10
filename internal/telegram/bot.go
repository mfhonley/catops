package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	constants "catops/config"
	"catops/internal/config"
	"catops/internal/metrics"
	"catops/internal/process"
	"catops/pkg/utils"
)

// formatBytes converts bytes to human-readable format (KB, MB, GB, etc.)
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncateCommand shortens long command strings for better display
func truncateCommand(command string, maxLen int) string {
	if len(command) <= maxLen {
		return command
	}
	return command[:maxLen-3] + "..."
}

// SendToTelegram sends a formatted message to a specific Telegram chat
func SendToTelegram(token string, chatID int64, message string) error {
	if token == "" || chatID == 0 {
		return fmt.Errorf("telegram not configured")
	}

	url := fmt.Sprintf(constants.TELEGRAM_API_URL, token)

	data := map[string]interface{}{
		"chat_id":                  chatID,
		"text":                     message,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}

	jsonData, _ := json.Marshal(data)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		// Log the error
		if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer f.Close()
			f.WriteString(fmt.Sprintf("[%s] ERROR: Telegram send error: %v\n", time.Now().Format("2006-01-02 15:04:05"), err))
		}
		return fmt.Errorf("telegram send error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// Log the error
		if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer f.Close()
			f.WriteString(fmt.Sprintf("[%s] ERROR: Telegram API error: %d\n", time.Now().Format("2006-01-02 15:04:05"), resp.StatusCode))
		}
		return fmt.Errorf("telegram API error: %d", resp.StatusCode)
	}

	return nil
}

// SetupBotCommands configures the bot's command menu in Telegram
func SetupBotCommands(bot *tgbotapi.BotAPI) error {
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Start monitoring service"},
		{Command: "status", Description: "Show current system metrics"},
		{Command: "processes", Description: "Show top processes by resource usage"},
		{Command: "restart", Description: "Restart monitoring service"},
		{Command: "set", Description: "Set alert thresholds (e.g., /set cpu=90)"},
		{Command: "version", Description: "Show CatOps version"},
		{Command: "help", Description: "Show available commands"},
	}

	_, err := bot.Request(tgbotapi.NewSetMyCommands(commands...))
	return err
}

// HandleBotCommand processes incoming bot commands and responds accordingly
func HandleBotCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, cfg *config.Config) {
	// Check if message is from the authorized group only
	if update.Message.Chat.ID != cfg.ChatID {
		// Send warning message for unauthorized chats
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		msg.ParseMode = "HTML"
		msg.Text = fmt.Sprintf(`🚫 <b>Unauthorized Group</b>

🤖 <b>This bot is configured for a specific group only!</b>

🌐 <b>This bot is created by catops.io</b>
• Visit <a href="%s">catops.io</a> for your own monitoring solution`, constants.CATOPS_WEBSITE)

		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	msg.ParseMode = "HTML"

	switch update.Message.Command() {
	case "start":
		// Show CatOps welcome message and features
		msg.Text = `🚀 <b>CatOps System Monitor</b>

📊 <b>Features:</b>
• Real-time Metrics - CPU, Memory, Disk monitoring
• Telegram Alerts - Instant notifications
• Telegram Bot - Remote control via commands
• Open Source - Free server monitoring solution
• Fast & Lightweight - Minimal resource usage

📋 <b>Quick Start Guide:</b>
1. Start Service - catops start
2. Set Thresholds - catops set cpu=90
3. Apply Changes - catops restart
4. Check Status - catops status
5. Telegram Bot - Automatically available

📝 <b>Available Commands:</b>
• status - Display current system metrics
• start - Start background monitoring service
• restart - Stop and restart monitoring service
• set - Configure alert thresholds and bot settings

💡 <b>For detailed help:</b> Use /help to see all bot commands

🔧 <b>Service Status:</b> ` + func() string {
			if process.IsRunning() {
				return "✅ Running"
			} else {
				return "❌ Stopped"
			}
		}()

	case "status":
		// Get current metrics
		metrics, err := metrics.GetMetrics()
		if err != nil {
			msg.Text = "❌ <b>Error getting metrics:</b>\n" + err.Error()
		} else {
			hostname, _ := os.Hostname()

			msg.Text = fmt.Sprintf(`📊 <b>System Status</b>

📋 <b>Server Information:</b>
• Hostname: %s
• OS: %s
• IP: %s
• Uptime: %s

📈 <b>Current Metrics:</b>
• CPU: %.1f%% (%d cores, %d active)
• Memory: %.1f%% (%s / %s)
• Disk: %.1f%% (%s / %s)
• HTTPS Connections: %d
• IOPS: %d
• I/O Wait: %.1f%%

🔔 <b>Alert Thresholds:</b>
• CPU: %.1f%%
• Memory: %.1f%%
• Disk: %.1f%%

⏰ <b>Last Updated:</b> %s`,
				hostname, metrics.OSName, metrics.IPAddress, metrics.Uptime,
				metrics.CPUUsage, metrics.CPUDetails.Total, metrics.CPUDetails.Used,
				metrics.MemoryUsage, utils.FormatBytes(metrics.MemoryDetails.Used*1024), utils.FormatBytes(metrics.MemoryDetails.Total*1024),
				metrics.DiskUsage, utils.FormatBytes(metrics.DiskDetails.Used*1024), utils.FormatBytes(metrics.DiskDetails.Total*1024),
				metrics.HTTPSRequests, metrics.IOPS, metrics.IOWait,
				cfg.CPUThreshold, cfg.MemThreshold, cfg.DiskThreshold,
				time.Now().Format("2006-01-02 15:04:05"))
		}

	case "processes":
		// Get current metrics with process information
		metrics, err := metrics.GetMetrics()
		if err != nil {
			msg.Text = "❌ <b>Error getting process information:</b>\n" + err.Error()
		} else {
			if len(metrics.TopProcesses) == 0 {
				msg.Text = "⚠️ <b>No process information available</b>\n\nTry again in a moment."
			} else {
				// Calculate total CPU usage of shown processes
				var totalCPU float64
				limit := 5
				if len(metrics.TopProcesses) < limit {
					limit = len(metrics.TopProcesses)
				}

				for i := 0; i < limit; i++ {
					totalCPU += metrics.TopProcesses[i].CPUUsage
				}

				// Format top processes for Telegram
				processText := fmt.Sprintf("🔍 <b>Top %d Processes</b>\n", limit)
				processText += fmt.Sprintf("📊 <b>Using %.1f%% of total system CPU</b>\n\n", totalCPU)

				for i := 0; i < limit; i++ {
					proc := metrics.TopProcesses[i]

					// Status emoji
					statusEmoji := "⚪" // Default
					switch proc.Status {
					case "R":
						statusEmoji = "🟢" // Running
					case "S":
						statusEmoji = "🟡" // Sleeping
					case "Z":
						statusEmoji = "🔴" // Zombie
					case "D":
						statusEmoji = "🔵" // Disk sleep
					}

					processText += fmt.Sprintf(
						"<b>%d.</b> <code>%s</code> %s\n"+
							"• PID: <code>%d</code> | User: <code>%s</code> | TTY: <code>%s</code>\n"+
							"• CPU: <b>%.1f%%</b> | Memory: <b>%.1f%%</b> (%s)\n"+
							"• Command: <code>%s</code>\n\n",
						i+1, proc.Name, statusEmoji,
						proc.PID, proc.User, proc.TTY,
						proc.CPUUsage, proc.MemoryUsage, formatBytes(proc.MemoryKB*1024),
						truncateCommand(proc.Command, 40))
				}

				processText += "💡 <b>Note:</b> CPU percentages are normalized to total system capacity\n"
				processText += fmt.Sprintf("⏰ <b>Last Updated:</b> %s", time.Now().Format("2006-01-02 15:04:05"))

				msg.Text = processText
			}
		}

	case "restart":
		// Restart monitoring service by running CLI command in background

		// Run catops restart in background
		cmd := exec.Command("catops", "restart")
		cmd.Dir = "/tmp"
		cmd.Stdout = nil
		cmd.Stderr = nil

		err := cmd.Start()
		if err != nil {
			if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("[%s] ERROR: Failed to start restart process: %v\n", time.Now().Format("2006-01-02 15:04:05"), err))
			}
			msg.Text = "❌ <b>Failed to restart:</b> " + err.Error()
		} else {

			msg.Text = "🔄 <b>Restarting CatOps...</b>\n\nMonitoring service is being restarted in the background.\n\n✅ <b>Status:</b> Restart process initiated"
		}

	case "set":
		// Handle threshold and config setting
		args := update.Message.CommandArguments()
		if args == "" {
			msg.Text = "📝 <b>Usage:</b>\n\n<b>Thresholds:</b>\n/set cpu=90\n/set mem=80\n/set disk=70\n\n<b>Bot Config:</b>\n/set token=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz\n/set group=123456789\n\n<b>Examples:</b>\n/set cpu=90 mem=80 disk=70\n/set token=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz"
		} else {
			// Parse arguments like "cpu=90 mem=80" or "token=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz"
			parts := strings.Fields(args)
			var updates []string

			for _, part := range parts {
				if strings.Contains(part, "=") {
					keyValue := strings.Split(part, "=")
					if len(keyValue) == 2 {
						key := keyValue[0]
						value := keyValue[1]

						switch key {
						case "cpu":
							if numValue, err := strconv.ParseFloat(value, 64); err == nil {
								cfg.CPUThreshold = numValue
								updates = append(updates, fmt.Sprintf("CPU: %.1f%%", numValue))
							}
						case "mem":
							if numValue, err := strconv.ParseFloat(value, 64); err == nil {
								cfg.MemThreshold = numValue
								updates = append(updates, fmt.Sprintf("Memory: %.1f%%", numValue))
							}
						case "disk":
							if numValue, err := strconv.ParseFloat(value, 64); err == nil {
								cfg.DiskThreshold = numValue
								updates = append(updates, fmt.Sprintf("Disk: %.1f%%", numValue))
							}
						case "token":
							if len(value) >= 20 {
								cfg.TelegramToken = value
								updates = append(updates, "Bot Token: Updated")
							}
						case "group":
							if numValue, err := strconv.ParseInt(value, 10, 64); err == nil {
								cfg.ChatID = numValue
								updates = append(updates, fmt.Sprintf("Group ID: %d", numValue))
							}
						}
					}
				}
			}

			if len(updates) > 0 {
				config.SaveConfig(cfg)
				msg.Text = fmt.Sprintf("✅ <b>Configuration updated:</b>\n%s\n\n💡 <b>Tip:</b> Use /restart to apply changes immediately.", strings.Join(updates, "\n"))
			} else {
				msg.Text = "❌ <b>Invalid format!</b>\n\nUsage: /set cpu=90 mem=80 disk=70\nOr: /set token=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz group=123456789"
			}
		}

	case "version":
		// Get CatOps version - use absolute path for daemon compatibility
		exePath, err := os.Executable()
		if err != nil {
			msg.Text = "❌ <b>Error getting executable path:</b>\n" + err.Error()
			break
		}

		cmd := exec.Command(exePath, "--version")
		output, err := cmd.Output()
		if err != nil {
			msg.Text = "❌ <b>Error getting version:</b>\n" + err.Error()
		} else {
			// Extract only version line (first line)
			fullOutput := strings.TrimSpace(string(output))
			lines := strings.Split(fullOutput, "\n")
			version := strings.TrimSpace(lines[0])
			// Remove "CatOps " prefix if present
			version = strings.TrimPrefix(version, "CatOps ")

			// Check for updates
			// Логируем начало запроса проверки версии
			if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("[%s] INFO: Version check request started from Telegram bot - URL: %s\n",
					time.Now().Format("2006-01-02 15:04:05"), constants.VERSIONS_URL))
			}

			// Create request with proper CLI headers
			req, err := utils.CreateCLIRequest("GET", constants.VERSIONS_URL, nil, version)
			var updateInfo string
			if err != nil {
				updateInfo = "\n\n❌ <b>Update check failed</b>"
			} else {
				client := &http.Client{Timeout: 10 * time.Second}
				resp, err := client.Do(req)
				if err == nil {
					defer resp.Body.Close()
					var result map[string]interface{}
					if json.NewDecoder(resp.Body).Decode(&result) == nil {
						// API returns "version" field, not "latest_version"
						if latestVersion, ok := result["version"].(string); ok {
							// Normalize both versions by removing "v" prefix for comparison
							currentVersion := strings.TrimPrefix(version, "v")
							normalizedLatest := strings.TrimPrefix(latestVersion, "v")

							if normalizedLatest != currentVersion {
								updateInfo = fmt.Sprintf("\n\n🔄 <b>Update available:</b> <code>v%s</code>\n💡 <b>To update:</b>\n<code>catops update</code>", normalizedLatest)
							} else {
								updateInfo = "\n\n✅ <b>You have the latest version!</b>"
							}
						}
					}
				} else {
					updateInfo = "\n\n❌ <b>Update check failed</b>"
				}
			}

			msg.Text = fmt.Sprintf("📦 <b>CatOps Version</b>\n\n<code>%s</code>%s\n\n💬 <b>Support:</b> @mfhonley", version, updateInfo)
		}

	case "update":
		// Update command removed from bot - use CLI instead
		msg.Text = "❌ <b>Update command removed from bot</b>\n\n💡 <b>Use CLI instead:</b>\n\n<code>catops update</code>\n\nThis is more reliable and secure."

	case "help":
		msg.Text = fmt.Sprintf(`🤖 <b>CatOps Bot Commands</b>

📋 <b>Available Commands:</b>
/start - Start monitoring service
/status - Show detailed system metrics with exact values
/processes - Show top processes by resource usage
/restart - Restart monitoring service
/set - Set alert thresholds and bot config
/version - Show CatOps version
/help - Show this help message

💡 <b>Examples:</b>
<b>Thresholds:</b>
/set cpu=90 mem=80 disk=70

<b>Bot Configuration:</b>
/set token=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
/set group=123456789

🔒 <b>Security:</b>
• Bot responds only in the configured group
• All actions are logged for security monitoring

🔧 <b>Note:</b> Changes made with /set require /restart to take effect immediately.

🌐 <b>Official Website:</b> <a href="%s">catops.io</a>`, constants.CATOPS_WEBSITE)

	default:
		msg.Text = "❓ <b>Unknown command!</b>\n\nUse /help to see available commands."
	}

	bot.Send(msg)
}

// StartTelegramBot starts the Telegram bot
func StartTelegramBot(cfg *config.Config) {
	if cfg.TelegramToken == "" {
		fmt.Println("❌ Error: Telegram token not configured!")
		fmt.Println("Please set telegram_token and group_id in ~/.catops/config.yaml")
		return
	}

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		fmt.Printf("❌ Error creating bot: %v\n", err)
		return
	}

	// Setup bot commands
	if err := SetupBotCommands(bot); err != nil {
		fmt.Printf("⚠️ Warning: Could not setup bot commands: %v\n", err)
	}

	fmt.Printf("🤖 Bot started: @%s\n", bot.Self.UserName)
	fmt.Println("📱 Bot is now listening for commands...")
	fmt.Println("💡 Use /help in Telegram to see available commands")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Log attempts to use bot in unauthorized groups
		if update.Message.IsCommand() && update.Message.Chat.ID != cfg.ChatID {
			// Log security attempt
			if f, err := os.OpenFile(constants.LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				f.WriteString(fmt.Sprintf("[%s] SECURITY: Bot command attempted in unauthorized group %d from user %d (%s %s)\n",
					time.Now().Format("2006-01-02 15:04:05"),
					update.Message.Chat.ID,
					update.Message.From.ID,
					update.Message.From.FirstName,
					update.Message.From.LastName))
				f.Close()
			}
		}

		if update.Message.IsCommand() {
			HandleBotCommand(bot, update, cfg)
		}
	}
}

// StartBotInBackground starts the bot in background
func StartBotInBackground(cfg interface{}) {
	// Convert interface{} to *config.Config
	config, ok := cfg.(*config.Config)
	if !ok {
		// Try to get config from viper
		return
	}

	if config.TelegramToken == "" {
		return // Silently skip if no token configured
	}

	go func() {
		StartTelegramBot(config)
	}()
}
