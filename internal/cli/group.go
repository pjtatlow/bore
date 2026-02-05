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
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a tunnel group",
		Long:  "Start all tunnels in a group.",
		Args:  cobra.ExactArgs(1),
		RunE:  runGroupEnable,
	}
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

	if !ipc.IsDaemonRunning() {
		return fmt.Errorf("daemon is not running (start with 'bore start')")
	}

	client, err := ipc.NewClient()
	if err != nil {
		return err
	}

	if err := client.GroupEnable(groupName); err != nil {
		return fmt.Errorf("failed to enable group '%s': %w", groupName, err)
	}

	fmt.Printf("Enabled group '%s'\n", groupName)
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
