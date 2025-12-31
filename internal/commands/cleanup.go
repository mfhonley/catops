package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"catops/internal/ui"
)

// NewCleanupCmd creates the cleanup command
func NewCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old backup files",
		Long: `Clean up old backup files created during updates.
This will remove specific old backup files and clean up files older than 30 days.

Examples:
  catops cleanup          # Clean up old backups`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Cleaning Up Old Backups")

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
