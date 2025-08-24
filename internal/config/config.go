package config

import (
	"fmt"
	"os"

	constants "moniq/config"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	TelegramToken string  `mapstructure:"telegram_token"`
	ChatID        int64   `mapstructure:"chat_id"`
	AuthToken     string  `mapstructure:"auth_token"`
	ServerToken   string  `mapstructure:"server_token"`
	Mode          string  `mapstructure:"mode"`
	CPUThreshold  float64 `mapstructure:"cpu_threshold"`
	MemThreshold  float64 `mapstructure:"mem_threshold"`
	DiskThreshold float64 `mapstructure:"disk_threshold"`
}

// determineMode automatically sets the operation mode based on tokens
func (cfg *Config) determineMode() {
	if cfg.AuthToken != "" && cfg.ServerToken != "" {
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
	configDir := os.Getenv("HOME") + "/.moniq"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Создаем содержимое конфига вручную
	configContent := fmt.Sprintf(`telegram_token: %s
chat_id: %d
auth_token: %s
server_token: %s
mode: %s
cpu_threshold: %.1f
mem_threshold: %.1f
disk_threshold: %.1f
`, cfg.TelegramToken, cfg.ChatID, cfg.AuthToken, cfg.ServerToken, cfg.Mode, cfg.CPUThreshold, cfg.MemThreshold, cfg.DiskThreshold)

	// Записываем в файл
	configFile := configDir + "/config.yaml"
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		return err
	}
	return nil
}
