package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"catops/internal/config"
	"catops/internal/server"
	"catops/internal/ui"
)

// NewAuthCmd creates the auth command with all subcommands
func NewAuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
		Long: `Manage authentication for CatOps.

Commands:
  login    Login with authentication token
  logout   Logout and clear authentication
  status   Show authentication status`,
	}

	// Add subcommands
	authCmd.AddCommand(newLoginCmd())
	authCmd.AddCommand(newLogoutCmd())
	authCmd.AddCommand(newStatusAuthCmd())
	authCmd.AddCommand(newTokenCmd())

	return authCmd
}

// newLoginCmd creates the login subcommand
func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login [token]",
		Short: "Login with authentication token",
		Long: `Login to CatOps with your authentication token.

Examples:
  catops auth login your_token_here
  catops auth login eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Authentication")

			newToken := args[0]

			// Load current config
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load config")
				return
			}

			// if we already have server_id, transfer ownership
			if cfg.ServerID != "" && cfg.AuthToken != "" {
				ui.PrintStatus("info", "Server is already registered, transferring ownership...")

				if !server.TransferServerOwnership(cfg.AuthToken, newToken, cfg.ServerID, GetCurrentVersion()) {
					ui.PrintStatus("error", "Failed to transfer server ownership")
					ui.PrintStatus("info", "Please check your token and try again")
					ui.PrintSectionEnd()
					return
				}

				ui.PrintStatus("success", "Server ownership transferred successfully")
			} else {
				// first time logging in - register server
				ui.PrintStatus("info", "Registering server with your account...")

				if !server.RegisterServer(newToken, GetCurrentVersion(), cfg) {
					ui.PrintStatus("error", "Failed to register server")
					ui.PrintStatus("info", "Please check your token and try again")
					ui.PrintSectionEnd()
					return
				}

				ui.PrintStatus("success", "Server registered successfully")
			}

			// update auth_token (server_id is already saved in registerServer)
			cfg.AuthToken = newToken
			if err := config.SaveConfig(cfg); err != nil {
				ui.PrintStatus("error", "Failed to save authentication token")
				return
			}

			ui.PrintStatus("success", "Authentication successful")
			ui.PrintSectionEnd()
		},
	}
}

// newLogoutCmd creates the logout subcommand
func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Logout and clear authentication",
		Long:  `Logout from CatOps and clear authentication token.`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Authentication")

			// load current config
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load config")
				return
			}

			// clear auth token
			cfg.AuthToken = ""

			// save config
			if err := config.SaveConfig(cfg); err != nil {
				ui.PrintStatus("error", "Failed to clear authentication token")
				return
			}

			ui.PrintStatus("success", "Logged out successfully")
			ui.PrintStatus("info", "Authentication token cleared")
			ui.PrintSectionEnd()
		},
	}
}

// newStatusAuthCmd creates the status subcommand
func newStatusAuthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show authentication status",
		Long:  `Display current authentication status and server registration status.`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Authentication Status")

			// load current config
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load config")
				return
			}

			if cfg.AuthToken != "" {
				ui.PrintStatus("success", "Authenticated")

				// Show shortened token instead of full JWT
				token := cfg.AuthToken
				if len(token) > 30 {
					token = token[:15] + "..." + token[len(token)-15:]
				}
				ui.PrintStatus("info", "Token: "+token)

				ui.PrintStatus("info", "Server registered: "+func() string {
					if cfg.ServerID != "" {
						return "Yes"
					}
					return "No"
				}())
			} else {
				ui.PrintStatus("warning", "Not authenticated")
				ui.PrintStatus("info", "Run 'catops auth login <token>' to authenticate")
			}

			ui.PrintSectionEnd()
		},
	}
}

// newTokenCmd creates the token subcommand
func newTokenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Show current authentication token",
		Long: `Display the current authentication token.

This command shows the full token that is currently stored in the configuration.`,
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintHeader()
			ui.PrintSection("Authentication Token")

			// load current config
			cfg, err := config.LoadConfig()
			if err != nil {
				ui.PrintStatus("error", "Failed to load config")
				return
			}

			if cfg.AuthToken != "" {
				ui.PrintStatus("success", "Current token:")
				fmt.Printf("  %s\n", cfg.AuthToken)
			} else {
				ui.PrintStatus("warning", "No authentication token found")
				ui.PrintStatus("info", "Run 'catops auth login <token>' to set a token")
			}

			ui.PrintSectionEnd()
		},
	}
}
