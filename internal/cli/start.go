package cli

import (
	"fmt"
	"time"

	"github.com/pjtatlow/bore/internal/daemon"
	"github.com/pjtatlow/bore/internal/ipc"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the bore daemon",
		Long:  "Start the bore daemon in the background. The daemon manages all SSH tunnels.",
		RunE:  runStart,
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	// If we're the daemon process, run the daemon
	if daemon.IsDaemon() {
		d, err := daemon.New()
		if err != nil {
			return err
		}
		return d.Run()
	}

	// Check if already running
	if ipc.IsDaemonRunning() {
		fmt.Println("Daemon is already running")
		return nil
	}

	// Fork to background
	if err := daemon.Fork(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for daemon to start
	fmt.Print("Starting daemon")
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if ipc.IsDaemonRunning() {
			fmt.Println(" done")
			return nil
		}
		fmt.Print(".")
	}

	fmt.Println(" timeout")
	return fmt.Errorf("daemon failed to start (check logs with 'bore logs')")
}
