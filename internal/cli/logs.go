package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/pjtatlow/bore/internal/ipc"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View daemon logs",
		Long:  "View or follow the daemon log file.",
		RunE:  runLogs,
	}

	cmd.Flags().BoolP("follow", "f", false, "Follow the log file (like tail -f)")
	cmd.Flags().IntP("lines", "n", 50, "Number of lines to show")

	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	logPath, err := ipc.LogPath()
	if err != nil {
		return err
	}

	follow, _ := cmd.Flags().GetBool("follow")
	lines, _ := cmd.Flags().GetInt("lines")

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Println("No log file found. Start the daemon with 'bore start' first.")
		return nil
	}

	if follow {
		return tailFollow(logPath, lines)
	}

	return tailLines(logPath, lines)
}

// tailLines shows the last n lines of a file
func tailLines(path string, n int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read all lines (simple approach for small log files)
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Print last n lines
	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	for _, line := range lines[start:] {
		fmt.Println(line)
	}

	return nil
}

// tailFollow follows the log file like tail -f
func tailFollow(path string, initialLines int) error {
	// First, show initial lines
	if err := tailLines(path, initialLines); err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Seek to end
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	fmt.Println("--- Following log file (Ctrl+C to stop) ---")

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Wait for more data
				continue
			}
			return err
		}
		fmt.Print(line)
	}
}
