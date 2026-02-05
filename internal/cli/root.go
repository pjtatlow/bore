package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "bore",
		Short: "SSH tunnel manager",
		Long:  "Bore is a self-managed SSH tunnel daemon with automatic reconnection and group-based tunnel management.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default action: run interactive selector
			return runInteractive()
		},
	}

	// Add subcommands
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newGroupCmd())
	rootCmd.AddCommand(newTunnelCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newLogsCmd())

	return rootCmd
}

// Execute runs the CLI
func Execute() error {
	return NewRootCmd().Execute()
}
