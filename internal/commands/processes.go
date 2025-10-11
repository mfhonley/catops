package commands

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"catops/internal/metrics"
	"catops/internal/ui"
)

// NewProcessesCmd creates the processes command
func NewProcessesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "processes",
		Short: "Show detailed information about running processes",
		Long: `Display detailed information about system processes including:
  • Top processes by CPU usage
  • Top processes by memory usage
  • Process details (PID, user, command, resource usage)

Examples:
  catops processes        # Show all process information
  catops processes -n 20 # Show top 20 processes`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Process Information")

			// get metrics with process information
			currentMetrics, err := metrics.GetMetrics()
			if err != nil {
				ui.PrintStatus("error", fmt.Sprintf("Error getting metrics: %v", err))
				ui.PrintSectionEnd()
				return
			}

			// get limit from flags
			limit, _ := cmd.Flags().GetInt("limit")

			// show top processes by CPU
			ui.PrintSection("Top Processes by CPU Usage")
			if len(currentMetrics.TopProcesses) > 0 {
				// sort by CPU usage
				sortedProcesses := make([]metrics.ProcessInfo, len(currentMetrics.TopProcesses))
				copy(sortedProcesses, currentMetrics.TopProcesses)
				sort.Slice(sortedProcesses, func(i, j int) bool {
					return sortedProcesses[i].CPUUsage > sortedProcesses[j].CPUUsage
				})

				// show top N processes
				if limit < len(sortedProcesses) {
					sortedProcesses = sortedProcesses[:limit]
				}

				fmt.Print(ui.CreateProcessTable(sortedProcesses))
			} else {
				ui.PrintStatus("warning", "No process information available")
			}
			ui.PrintTableSectionEnd()

			// show top processes by memory
			ui.PrintSection("Top Processes by Memory Usage")
			if len(currentMetrics.TopProcesses) > 0 {
				// sort by memory usage
				sortedProcesses := make([]metrics.ProcessInfo, len(currentMetrics.TopProcesses))
				copy(sortedProcesses, currentMetrics.TopProcesses)
				sort.Slice(sortedProcesses, func(i, j int) bool {
					return sortedProcesses[i].MemoryUsage > sortedProcesses[j].MemoryUsage
				})

				// show top N processes
				if limit < len(sortedProcesses) {
					sortedProcesses = sortedProcesses[:limit]
				}

				fmt.Print(ui.CreateProcessTableByMemory(sortedProcesses))
			} else {
				ui.PrintStatus("warning", "No process information available")
			}
			ui.PrintTableSectionEnd()
		},
	}

	cmd.Flags().IntP("limit", "n", 10, "Number of processes to show")

	return cmd
}
