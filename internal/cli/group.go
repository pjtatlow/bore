package cli

import (
	"fmt"

	"github.com/pjtatlow/bore/internal/ipc"
	"github.com/spf13/cobra"
)

func newGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage tunnel groups",
		Long:  "Enable or disable tunnel groups.",
	}

	cmd.AddCommand(newGroupEnableCmd())
	cmd.AddCommand(newGroupDisableCmd())

	return cmd
}

func newGroupEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a tunnel group",
		Long:  "Start all tunnels in a group, connecting through the specified host.",
		Args:  cobra.ExactArgs(1),
		RunE:  runGroupEnable,
	}
	cmd.Flags().String("host", "", "SSH host to connect through (required)")
	cmd.MarkFlagRequired("host")
	return cmd
}

func newGroupDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a tunnel group",
		Long:  "Stop all tunnels in a group.",
		Args:  cobra.ExactArgs(1),
		RunE:  runGroupDisable,
	}
}

func runGroupEnable(cmd *cobra.Command, args []string) error {
	groupName := args[0]
	host, _ := cmd.Flags().GetString("host")

	if !ipc.IsDaemonRunning() {
		return fmt.Errorf("daemon is not running (start with 'bore start')")
	}

	client, err := ipc.NewClient()
	if err != nil {
		return err
	}

	if err := client.GroupEnable(groupName, host); err != nil {
		return fmt.Errorf("failed to enable group '%s': %w", groupName, err)
	}

	fmt.Printf("Enabled group '%s' via host '%s'\n", groupName, host)
	return nil
}

func runGroupDisable(cmd *cobra.Command, args []string) error {
	groupName := args[0]

	if !ipc.IsDaemonRunning() {
		return fmt.Errorf("daemon is not running (start with 'bore start')")
	}

	client, err := ipc.NewClient()
	if err != nil {
		return err
	}

	if err := client.GroupDisable(groupName); err != nil {
		return fmt.Errorf("failed to disable group '%s': %w", groupName, err)
	}

	fmt.Printf("Disabled group '%s'\n", groupName)
	return nil
}
