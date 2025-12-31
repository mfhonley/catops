package commands

import (
	"os/exec"

	"github.com/spf13/cobra"

	"catops/internal/service"
	"catops/internal/ui"
)

// NewForceCleanupCmd creates the force-cleanup command
func NewForceCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "force-cleanup",
		Short: "Force stop all CatOps processes",
		Long: `Force stop all CatOps daemon processes.
This command will stop the service and kill any remaining catops processes.
Use this when you need a fresh start.

Examples:
  catops force-cleanup    # Stop all processes`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Force Cleanup")

			ui.PrintStatus("warning", "Stopping all CatOps processes...")

			// Try to stop via service manager first
			svc, err := service.New()
			if err == nil {
				svc.Stop()
			}

			// Also kill any remaining processes via pkill
			killCmd := exec.Command("pkill", "-9", "-f", "catops daemon")
			killCmd.Run() // Ignore errors

			ui.PrintStatus("success", "Force cleanup completed.")
			ui.PrintStatus("info", "Run 'catops service start' to start fresh.")
			ui.PrintSectionEnd()
		},
	}
}
