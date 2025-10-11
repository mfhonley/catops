package commands

import (
	"github.com/spf13/cobra"

	"catops/internal/process"
	"catops/internal/ui"
)

// NewForceCleanupCmd creates the force-cleanup command
func NewForceCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "force-cleanup",
		Short: "Force cleanup of all duplicate processes and zombie processes",
		Long: `Force cleanup of all duplicate catops processes and zombie processes.
This command will kill ALL catops daemon processes and clean up any zombie processes.
Use this when you have multiple processes running and need a fresh start.

Examples:
  catops force-cleanup    # Kill all processes and start fresh`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Force Cleanup of All Processes")

			ui.PrintStatus("warning", "This will kill ALL catops daemon processes!")

			// kill all catops daemon processes
			process.KillAllCatOpsProcesses()

			ui.PrintStatus("success", "Force cleanup completed. All processes killed.")
			ui.PrintStatus("info", "Run 'catops start' to start fresh monitoring service.")
			ui.PrintSectionEnd()
		},
	}
}
