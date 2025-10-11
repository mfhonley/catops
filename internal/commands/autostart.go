package commands

import (
	"os"

	"github.com/spf13/cobra"

	"catops/internal/autostart"
	"catops/internal/ui"
)

// NewAutostartCmd creates the autostart command
func NewAutostartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "autostart",
		Short: "Enable or disable autostart on boot",
		Long: `Enable or disable autostart on boot.
This creates systemd service (Linux) or launchd service (macOS) to start catops automatically.
Examples:
  catops autostart enable   # Enable autostart
  catops autostart disable  # Disable autostart
  catops autostart status   # Check autostart status`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Autostart Management")

			if len(args) == 0 {
				ui.PrintStatus("error", "Please specify: enable, disable, or status")
				ui.PrintSectionEnd()
				return
			}

			action := args[0]
			executable, err := os.Executable()
			if err != nil {
				ui.PrintStatus("error", "Could not determine binary location")
				ui.PrintSectionEnd()
				return
			}

			switch action {
			case "enable":
				autostart.Enable(executable)
			case "disable":
				autostart.Disable()
			case "status":
				autostart.CheckStatus()
			default:
				ui.PrintStatus("error", "Invalid action. Use: enable, disable, or status")
			}

			ui.PrintSectionEnd()
		},
	}
}
