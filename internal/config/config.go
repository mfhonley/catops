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
	SuddenSpikeThreshold   float64 `mapstructure:"sudden_spike_threshold"`   // default 30.0%
	GradualRiseThreshold   float64 `mapstructure:"gradual_rise_threshold"`   // default 15.0%
	AnomalyThreshold       float64 `mapstructure:"anomaly_threshold"`        // default 4.0 (standard deviations)
	AlertDeduplication     bool    `mapstructure:"alert_deduplication"`      // default true
	AlertRenotifyInterval  int     `mapstructure:"alert_renotify_interval"`  // in minutes, default 120
	AlertResolutionTimeout int     `mapstructure:"alert_resolution_timeout"` // in minutes, default 5

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
	viper.SetDefault("collection_interval", constants.DEFAULT_COLLECTION_INTERVAL)
	viper.SetDefault("buffer_size", constants.DEFAULT_BUFFER_SIZE)
	viper.SetDefault("sudden_spike_threshold", constants.DEFAULT_SUDDEN_SPIKE_THRESHOLD)
	viper.SetDefault("gradual_rise_threshold", constants.DEFAULT_GRADUAL_RISE_THRESHOLD)
	viper.SetDefault("anomaly_threshold", constants.DEFAULT_ANOMALY_THRESHOLD)
	viper.SetDefault("alert_deduplication", constants.DEFAULT_ALERT_DEDUPLICATION)
	viper.SetDefault("alert_renotify_interval", constants.DEFAULT_ALERT_RENOTIFY_INTERVAL)
	viper.SetDefault("alert_resolution_timeout", constants.DEFAULT_ALERT_RESOLUTION_TIMEOUT)

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

	// Monitoring configuration (save if non-default)
	if cfg.CollectionInterval > 0 && cfg.CollectionInterval != constants.DEFAULT_COLLECTION_INTERVAL {
		configLines = append(configLines, "")
		configLines = append(configLines, "# Monitoring configuration")
		configLines = append(configLines, fmt.Sprintf("collection_interval: %d", cfg.CollectionInterval))
	}
	if cfg.BufferSize > 0 && cfg.BufferSize != constants.DEFAULT_BUFFER_SIZE {
		if cfg.CollectionInterval == 0 || cfg.CollectionInterval == constants.DEFAULT_COLLECTION_INTERVAL {
			configLines = append(configLines, "")
			configLines = append(configLines, "# Monitoring configuration")
		}
		configLines = append(configLines, fmt.Sprintf("buffer_size: %d", cfg.BufferSize))
	}
	if cfg.AlertResolutionTimeout > 0 && cfg.AlertResolutionTimeout != constants.DEFAULT_ALERT_RESOLUTION_TIMEOUT {
		if (cfg.CollectionInterval == 0 || cfg.CollectionInterval == constants.DEFAULT_COLLECTION_INTERVAL) &&
			(cfg.BufferSize == 0 || cfg.BufferSize == constants.DEFAULT_BUFFER_SIZE) {
			configLines = append(configLines, "")
			configLines = append(configLines, "# Monitoring configuration")
		}
		configLines = append(configLines, fmt.Sprintf("alert_resolution_timeout: %d", cfg.AlertResolutionTimeout))
	}

	// Spike detection thresholds (save if non-default)
	if cfg.SuddenSpikeThreshold > 0 && cfg.SuddenSpikeThreshold != constants.DEFAULT_SUDDEN_SPIKE_THRESHOLD {
		configLines = append(configLines, "")
		configLines = append(configLines, "# Alert sensitivity configuration")
		configLines = append(configLines, fmt.Sprintf("sudden_spike_threshold: %.1f", cfg.SuddenSpikeThreshold))
	}
	if cfg.GradualRiseThreshold > 0 && cfg.GradualRiseThreshold != constants.DEFAULT_GRADUAL_RISE_THRESHOLD {
		if cfg.SuddenSpikeThreshold == 0 || cfg.SuddenSpikeThreshold == constants.DEFAULT_SUDDEN_SPIKE_THRESHOLD {
			configLines = append(configLines, "")
			configLines = append(configLines, "# Alert sensitivity configuration")
		}
		configLines = append(configLines, fmt.Sprintf("gradual_rise_threshold: %.1f", cfg.GradualRiseThreshold))
	}
	if cfg.AnomalyThreshold > 0 && cfg.AnomalyThreshold != constants.DEFAULT_ANOMALY_THRESHOLD {
		if (cfg.SuddenSpikeThreshold == 0 || cfg.SuddenSpikeThreshold == constants.DEFAULT_SUDDEN_SPIKE_THRESHOLD) &&
			(cfg.GradualRiseThreshold == 0 || cfg.GradualRiseThreshold == constants.DEFAULT_GRADUAL_RISE_THRESHOLD) {
			configLines = append(configLines, "")
			configLines = append(configLines, "# Alert sensitivity configuration")
		}
		configLines = append(configLines, fmt.Sprintf("anomaly_threshold: %.1f", cfg.AnomalyThreshold))
	}
	if cfg.AlertRenotifyInterval > 0 && cfg.AlertRenotifyInterval != constants.DEFAULT_ALERT_RENOTIFY_INTERVAL {
		if (cfg.SuddenSpikeThreshold == 0 || cfg.SuddenSpikeThreshold == constants.DEFAULT_SUDDEN_SPIKE_THRESHOLD) &&
			(cfg.GradualRiseThreshold == 0 || cfg.GradualRiseThreshold == constants.DEFAULT_GRADUAL_RISE_THRESHOLD) {
			configLines = append(configLines, "")
			configLines = append(configLines, "# Alert sensitivity configuration")
		}
		configLines = append(configLines, fmt.Sprintf("alert_renotify_interval: %d", cfg.AlertRenotifyInterval))
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
