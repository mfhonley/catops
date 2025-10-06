package config

import (
	"fmt"
	"os"

	constants "catops/config"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	TelegramToken string  `mapstructure:"telegram_token"`
	ChatID        int64   `mapstructure:"chat_id"`
	AuthToken     string  `mapstructure:"auth_token"`
	ServerID      string  `mapstructure:"server_id"`
	Mode          string  `mapstructure:"mode"`
	CPUThreshold  float64 `mapstructure:"cpu_threshold"`
	MemThreshold  float64 `mapstructure:"mem_threshold"`
	DiskThreshold float64 `mapstructure:"disk_threshold"`
}

// determineMode automatically sets the operation mode based on tokens
func (cfg *Config) determineMode() {
	if cfg.AuthToken != "" && cfg.ServerID != "" {
		cfg.Mode = constants.MODE_CLOUD
	} else {
		cfg.Mode = constants.MODE_LOCAL
	}
}

// IsCloudMode checks if the CLI is running in cloud mode
func (cfg *Config) IsCloudMode() bool {
	return cfg.Mode == constants.MODE_CLOUD
}

// IsLocalMode checks if the CLI is running in local mode
func (cfg *Config) IsLocalMode() bool {
	return cfg.Mode == constants.MODE_LOCAL
}

// LoadConfig loads configuration from file and environment
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME" + constants.CONFIG_DIR_NAME)
	viper.AddConfigPath(".")

	// Set defaults
	viper.SetDefault("cpu_threshold", constants.DEFAULT_CPU_THRESHOLD)
	viper.SetDefault("mem_threshold", constants.DEFAULT_MEMORY_THRESHOLD)
	viper.SetDefault("disk_threshold", constants.DEFAULT_DISK_THRESHOLD)

	// Read config file
	viper.ReadInConfig()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Determine operation mode
	cfg.determineMode()

	return &cfg, nil
}

// SaveConfig saves configuration to file
func SaveConfig(cfg *Config) error {
	configDir := os.Getenv("HOME") + "/.catops"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Build config content with only non-empty values
	var configLines []string

	// Telegram settings (optional)
	if cfg.TelegramToken != "" {
		configLines = append(configLines, fmt.Sprintf("telegram_token: %s", cfg.TelegramToken))
	}
	if cfg.ChatID != 0 {
		configLines = append(configLines, fmt.Sprintf("chat_id: %d", cfg.ChatID))
	}

	// Cloud mode settings
	if cfg.AuthToken != "" {
		configLines = append(configLines, fmt.Sprintf("auth_token: %s", cfg.AuthToken))
	}
	if cfg.ServerID != "" {
		configLines = append(configLines, fmt.Sprintf("server_id: %s", cfg.ServerID))
	}

	// Alert thresholds (always save)
	configLines = append(configLines, fmt.Sprintf("cpu_threshold: %.1f", cfg.CPUThreshold))
	configLines = append(configLines, fmt.Sprintf("mem_threshold: %.1f", cfg.MemThreshold))
	configLines = append(configLines, fmt.Sprintf("disk_threshold: %.1f", cfg.DiskThreshold))

	// Join lines with newline
	configContent := ""
	for i, line := range configLines {
		configContent += line
		if i < len(configLines)-1 {
			configContent += "\n"
		}
	}

	// Write to file
	configFile := configDir + "/config.yaml"
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		return err
	}
	return nil
}
