package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/pjtatlow/bore/internal/ipc"
)

const daemonEnvVar = "BORE_DAEMON"

// Fork starts the daemon as a background process
func Fork() error {
	// Get the path to the current executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Prepare log file
	logPath, err := ipc.LogPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create the daemon process
	cmd := exec.Command(exe, "start")
	cmd.Env = append(os.Environ(), daemonEnvVar+"=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = "/"

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	logFile.Close()
	return nil
}

// IsDaemon returns true if running as the daemon process
func IsDaemon() bool {
	return os.Getenv(daemonEnvVar) == "1"
}

// WritePID writes the current process ID to the PID file
func WritePID() error {
	pidPath, err := ipc.PIDPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(pidPath), 0700); err != nil {
		return fmt.Errorf("failed to create pid directory: %w", err)
	}

	pid := os.Getpid()
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0600)
}

// ReadPID reads the daemon PID from the PID file
func ReadPID() (int, error) {
	pidPath, err := ipc.PIDPath()
	if err != nil {
		return 0, err
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(data))
}

// RemovePID removes the PID file
func RemovePID() error {
	pidPath, err := ipc.PIDPath()
	if err != nil {
		return err
	}
	return os.Remove(pidPath)
}

// IsProcessRunning checks if a process with the given PID is running
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// StopDaemon sends a stop signal to the daemon
func StopDaemon() error {
	pid, err := ReadPID()
	if err != nil {
		return fmt.Errorf("daemon not running or PID file not found")
	}

	if !IsProcessRunning(pid) {
		// Clean up stale PID file
		RemovePID()
		return fmt.Errorf("daemon not running (stale PID file removed)")
	}

	// Try graceful shutdown via IPC first
	client, err := ipc.NewClient()
	if err == nil {
		if err := client.Stop(); err == nil {
			return nil
		}
	}

	// Fall back to SIGTERM
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Signal(syscall.SIGTERM)
}
