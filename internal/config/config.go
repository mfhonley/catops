package config

import (
	"fmt"
	"os"

	constants "catops/config"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	AuthToken     string  `mapstructure:"auth_token"`
	ServerID      string  `mapstructure:"server_id"`
	Mode          string  `mapstructure:"mode"`
	CPUThreshold  float64 `mapstructure:"cpu_threshold"`
	MemThreshold  float64 `mapstructure:"mem_threshold"`
	DiskThreshold float64 `mapstructure:"disk_threshold"`

	// Monitoring configuration
	CollectionInterval     int     `mapstructure:"collection_interval"`      // in seconds, default 15
	BufferSize             int     `mapstructure:"buffer_size"`              // default 20 (5 minutes at 15s)
	SuddenSpikeThreshold   float64 `mapstructure:"sudden_spike_threshold"`   // default 20%
	GradualRiseThreshold   float64 `mapstructure:"gradual_rise_threshold"`   // default 10%
	AlertDeduplication     bool    `mapstructure:"alert_deduplication"`      // default true
	AlertRenotifyInterval  int     `mapstructure:"alert_renotify_interval"`  // in minutes, default 60
	AlertResolutionTimeout int     `mapstructure:"alert_resolution_timeout"` // in minutes, default 2
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

	// Set defaults for thresholds
	viper.SetDefault("cpu_threshold", constants.DEFAULT_CPU_THRESHOLD)
	viper.SetDefault("mem_threshold", constants.DEFAULT_MEMORY_THRESHOLD)
	viper.SetDefault("disk_threshold", constants.DEFAULT_DISK_THRESHOLD)

	// Set defaults for monitoring configuration
	viper.SetDefault("collection_interval", 15)      // 15 seconds
	viper.SetDefault("buffer_size", 20)              // 20 points = 5 minutes at 15s
	viper.SetDefault("sudden_spike_threshold", 20.0) // 20% change
	viper.SetDefault("gradual_rise_threshold", 10.0) // 10% change over window
	viper.SetDefault("alert_deduplication", true)    // enabled
	viper.SetDefault("alert_renotify_interval", 60)  // 60 minutes
	viper.SetDefault("alert_resolution_timeout", 2)  // 2 minutes

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
