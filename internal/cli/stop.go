package cli

import (
	"fmt"
	"time"

	"github.com/pjtatlow/bore/internal/daemon"
	"github.com/pjtatlow/bore/internal/ipc"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the bore daemon",
		Long:  "Stop the bore daemon and all managed tunnels.",
		RunE:  runStop,
	}
}

func runStop(cmd *cobra.Command, args []string) error {
	if !ipc.IsDaemonRunning() {
		fmt.Println("Daemon is not running")
		return nil
	}

	fmt.Print("Stopping daemon")

	if err := daemon.StopDaemon(); err != nil {
		fmt.Println()
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	// Wait for daemon to stop
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !ipc.IsDaemonRunning() {
			fmt.Println(" done")
			return nil
		}
		fmt.Print(".")
	}

	fmt.Println(" timeout")
	return fmt.Errorf("daemon failed to stop")
}
