package cli

import (
	"fmt"

	"github.com/pjtatlow/bore/internal/ipc"
	"github.com/spf13/cobra"
)

func newTunnelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tunnel",
		Short: "Manage individual tunnels",
		Long:  "Start or stop individual tunnels.",
	}

	cmd.AddCommand(newTunnelUpCmd())
	cmd.AddCommand(newTunnelDownCmd())

	return cmd
}

func newTunnelUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up <name>",
		Short: "Start a tunnel",
		Long:  "Start an individual tunnel by name, connecting through the specified host.",
		Args:  cobra.ExactArgs(1),
		RunE:  runTunnelUp,
	}
	cmd.Flags().String("host", "", "SSH host to connect through (required)")
	cmd.MarkFlagRequired("host")
	return cmd
}

func newTunnelDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down <name>",
		Short: "Stop a tunnel",
		Long:  "Stop an individual tunnel by name.",
		Args:  cobra.ExactArgs(1),
		RunE:  runTunnelDown,
	}
}

func runTunnelUp(cmd *cobra.Command, args []string) error {
	tunnelName := args[0]
	host, _ := cmd.Flags().GetString("host")

	if !ipc.IsDaemonRunning() {
		return fmt.Errorf("daemon is not running (start with 'bore start')")
	}

	client, err := ipc.NewClient()
	if err != nil {
		return err
	}

	if err := client.TunnelUp(tunnelName, host); err != nil {
		return fmt.Errorf("failed to start tunnel '%s': %w", tunnelName, err)
	}

	fmt.Printf("Started tunnel '%s' via host '%s'\n", tunnelName, host)
	return nil
}

func runTunnelDown(cmd *cobra.Command, args []string) error {
	tunnelName := args[0]

	if !ipc.IsDaemonRunning() {
		return fmt.Errorf("daemon is not running (start with 'bore start')")
	}

	client, err := ipc.NewClient()
	if err != nil {
		return err
	}

	if err := client.TunnelDown(tunnelName); err != nil {
		return fmt.Errorf("failed to stop tunnel '%s': %w", tunnelName, err)
	}

	fmt.Printf("Stopped tunnel '%s'\n", tunnelName)
	return nil
}
