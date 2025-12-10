package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"catops/internal/config"
	"catops/internal/metrics"
	"catops/internal/process"
	"catops/internal/ui"
	"catops/pkg/utils"
)

// NewStatusCmd creates the status command
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Display current system metrics",
		Long: `Display real-time system information including:
  • System Information (Hostname, OS, IP, Uptime)
  • Current Metrics (CPU, Memory, Disk, HTTPS Connections)

Examples:
  catops status          # Show all system information`,
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load configuration")
				cfg = &config.Config{}
			}

			// get system information
			hostname, _ := os.Hostname()
			// Use cached metrics for faster response (avoids 1-second CPU measurement delay)
			currentMetrics, err := metrics.GetMetricsWithCache()
			if err != nil {
				ui.PrintStatus("error", fmt.Sprintf("Error getting metrics: %v", err))
				return
			}

			// system information section
			ui.PrintSection("System Information")
			systemData := map[string]string{
				"Hostname": hostname,
				"OS":       currentMetrics.OSName,
				"IP":       currentMetrics.IPAddress,
				"Uptime":   currentMetrics.Uptime,
			}
			fmt.Print(ui.CreateBeautifulList(systemData))
			ui.PrintSectionEnd()

			// timestamp section
			ui.PrintSection("Timestamp")
			timestampData := map[string]string{
				"Current Time": currentMetrics.Timestamp,
			}
			fmt.Print(ui.CreateBeautifulList(timestampData))
			ui.PrintSectionEnd()

			// metrics section
			ui.PrintSection("Current Metrics")
			metricsData := map[string]string{
				"CPU Usage":         fmt.Sprintf("%s (%d cores, %d active)", utils.FormatPercentage(currentMetrics.CPUUsage), currentMetrics.CPUDetails.Total, currentMetrics.CPUDetails.Used),
				"Memory Usage":      fmt.Sprintf("%s (%s / %s)", utils.FormatPercentage(currentMetrics.MemoryUsage), utils.FormatBytes(currentMetrics.MemoryDetails.Used*1024), utils.FormatBytes(currentMetrics.MemoryDetails.Total*1024)),
				"Disk Usage":        fmt.Sprintf("%s (%s / %s)", utils.FormatPercentage(currentMetrics.DiskUsage), utils.FormatBytes(currentMetrics.DiskDetails.Used*1024), utils.FormatBytes(currentMetrics.DiskDetails.Total*1024)),
				"HTTPS Connections": utils.FormatNumber(currentMetrics.HTTPSRequests),
				"IOPS":              utils.FormatNumber(currentMetrics.IOPS),
				"I/O Wait":          utils.FormatPercentage(currentMetrics.IOWait),
			}
			fmt.Print(ui.CreateBeautifulList(metricsData))
			ui.PrintSectionEnd()

			// monitoring settings section
			ui.PrintSection("Monitoring Settings")
			settingsData := map[string]string{
				"Collection Interval": fmt.Sprintf("%d seconds", cfg.CollectionInterval),
				"Mode":                cfg.Mode,
			}
			fmt.Print(ui.CreateBeautifulList(settingsData))
			ui.PrintSectionEnd()

			// daemon status
			ui.PrintSection("Daemon Status")
			if process.IsRunning() {
				ui.PrintStatus("success", "Monitoring daemon is running")
			} else {
				ui.PrintStatus("warning", "Monitoring daemon is not running")
			}
			ui.PrintSectionEnd()
		},
	}
}
