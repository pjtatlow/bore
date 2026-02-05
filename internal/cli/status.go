package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/pjtatlow/bore/internal/ipc"
	"github.com/pjtatlow/bore/internal/tunnel"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon and tunnel status",
		Long:  "Display the status of the daemon, all managed tunnels, and their statistics.",
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	if !ipc.IsDaemonRunning() {
		fmt.Println("Daemon is not running")
		return nil
	}

	client, err := ipc.NewClient()
	if err != nil {
		return err
	}

	status, err := client.Status()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	// Print daemon status
	fmt.Printf("Daemon: running (PID %d, uptime %s)\n", status.PID, status.Uptime)
	fmt.Printf("Network: %s\n", status.Network.Status)
	fmt.Println()

	// Print tunnels
	if len(status.Tunnels) == 0 {
		fmt.Println("No active tunnels")
	} else {
		fmt.Println("Tunnels:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tTYPE\tSTATUS\tLOCAL\tREMOTE\tTRAFFIC\tCONNS\tRECONNECTS")

		for _, t := range status.Tunnels {
			statusStr := formatStatus(t.Status)
			local := fmt.Sprintf("%d", t.LocalPort)
			remote := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
			traffic := formatBytes(t.BytesSent + t.BytesReceived)

			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				t.Name, t.Type, statusStr, local, remote, traffic, t.Connections, t.ReconnectCount)
		}
		w.Flush()
	}
	fmt.Println()

	// Print groups
	if len(status.Groups) > 0 {
		fmt.Println("Groups:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tSTATUS\tDESCRIPTION\tTUNNELS")

		for _, g := range status.Groups {
			statusStr := "disabled"
			if g.Enabled {
				statusStr = "enabled"
			}
			tunnelCount := fmt.Sprintf("%d tunnels", len(g.Tunnels))
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", g.Name, statusStr, g.Description, tunnelCount)
		}
		w.Flush()
	}

	return nil
}

func formatStatus(status tunnel.Status) string {
	switch status {
	case tunnel.StatusConnected:
		return "connected"
	case tunnel.StatusConnecting:
		return "connecting"
	case tunnel.StatusReconnecting:
		return "reconnecting"
	case tunnel.StatusError:
		return "error"
	case tunnel.StatusStopped:
		return "stopped"
	default:
		return string(status)
	}
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
