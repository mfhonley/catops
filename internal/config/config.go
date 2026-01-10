package config

import (
	"fmt"
	"os"

	constants "catops/config"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	AuthToken string `mapstructure:"auth_token"`
	ServerID  string `mapstructure:"server_id"`
	Mode      string `mapstructure:"mode"`

	// Monitoring configuration
	CollectionInterval int `mapstructure:"collection_interval"` // in seconds, default 15
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
	viper.AddConfigPath(os.Getenv("HOME") + constants.CONFIG_DIR_NAME)
	viper.AddConfigPath(".")

	// Set defaults for monitoring configuration
	viper.SetDefault("collection_interval", constants.DEFAULT_COLLECTION_INTERVAL)

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

	// Cloud mode settings
	if cfg.AuthToken != "" {
		configLines = append(configLines, fmt.Sprintf("auth_token: %s", cfg.AuthToken))
	}
	if cfg.ServerID != "" {
		configLines = append(configLines, fmt.Sprintf("server_id: %s", cfg.ServerID))
	}

	// Monitoring configuration (save if non-default)
	if cfg.CollectionInterval > 0 && cfg.CollectionInterval != constants.DEFAULT_COLLECTION_INTERVAL {
		configLines = append(configLines, "")
		configLines = append(configLines, "# Monitoring configuration")
		configLines = append(configLines, fmt.Sprintf("collection_interval: %d", cfg.CollectionInterval))
	}

	// Join lines with newline
	configContent := ""
	for i, line := range configLines {
		configContent += line
		if i < len(configLines)-1 {
			configContent += "\n"
		}
	}

	// Write to file with secure permissions (0600 - only owner can read/write)
	configFile := configDir + "/config.yaml"
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		return err
	}
	return nil
}
