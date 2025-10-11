package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"catops/internal/process"
	"catops/internal/ui"
)

// NewCleanupCmd creates the cleanup command
func NewCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old backup files and duplicate processes",
		Long: `Clean up old backup files created during updates and kill duplicate catops processes.
This will remove specific old backup files, clean up files older than 30 days, and ensure only one catops daemon is running.

Examples:
  catops cleanup          # Clean up old backups and processes`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Cleaning Up Old Backups and Processes")

			// clean up duplicate processes first
			process.KillDuplicateProcesses()
			process.CleanupZombieProcesses()
			ui.PrintStatus("success", "Process cleanup completed")

			// clean up old backup files
			executable, err := os.Executable()
			if err != nil {
				ui.PrintStatus("error", "Could not determine binary location")
				ui.PrintSectionEnd()
				return
			}
			backupDir := executable
			backupDir = backupDir[:len(backupDir)-len("/catops")] // Adjust path
			removedCount := 0
			for i := 3; i <= 10; i++ { // Remove specific old backups
				backupFile := fmt.Sprintf("%s/catops.backup.%d", backupDir, i)
				if _, err := os.Stat(backupFile); err == nil {
					os.Remove(backupFile)
					removedCount++
				}
			}
			cmd2 := exec.Command("find", backupDir, "-name", "catops.backup.*", "-mtime", "+30", "-delete")
			cmd2.Run() // Ignore errors
			ui.PrintStatus("success", fmt.Sprintf("Cleanup completed. Removed %d old backup files", removedCount))
			ui.PrintSectionEnd()
		},
	}
}
