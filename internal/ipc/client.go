package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Client communicates with the daemon via Unix socket
type Client struct {
	socketPath string
}

// NewClient creates a new IPC client
func NewClient() (*Client, error) {
	socketPath, err := SocketPath()
	if err != nil {
		return nil, err
	}
	return &Client{socketPath: socketPath}, nil
}

// SocketPath returns the path to the Unix socket
func SocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bore", "bore.sock"), nil
}

// PIDPath returns the path to the PID file
func PIDPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bore", "bore.pid"), nil
}

// LogPath returns the path to the log file
func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bore", "bore.log"), nil
}

// StatePath returns the path to the state file
func StatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bore", "state.json"), nil
}

// Send sends a request and returns the response
func (c *Client) Send(req Request) (*Response, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer conn.Close()

	// Set deadline for the entire operation
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Send request
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &resp, nil
}

// Ping checks if the daemon is running
func (c *Client) Ping() error {
	resp, err := c.Send(Request{Type: ReqPing})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("ping failed: %s", resp.Error)
	}
	return nil
}

// Status gets the daemon status
func (c *Client) Status() (*StatusResponse, error) {
	resp, err := c.Send(Request{Type: ReqStatus})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("status failed: %s", resp.Error)
	}

	// Decode the data into StatusResponse
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, err
	}
	var status StatusResponse
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// Stop tells the daemon to shut down
func (c *Client) Stop() error {
	resp, err := c.Send(Request{Type: ReqStop})
	if err != nil {
		// Connection closed is expected when daemon stops
		return nil
	}
	if !resp.Success {
		return fmt.Errorf("stop failed: %s", resp.Error)
	}
	return nil
}

// TunnelUp starts a tunnel
func (c *Client) TunnelUp(name, host string) error {
	resp, err := c.Send(Request{
		Type: ReqTunnelUp,
		Data: TunnelRequest{Name: name, Host: host},
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// TunnelDown stops a tunnel
func (c *Client) TunnelDown(name string) error {
	resp, err := c.Send(Request{
		Type: ReqTunnelDown,
		Data: TunnelRequest{Name: name},
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GroupEnable enables a tunnel group
func (c *Client) GroupEnable(name, host string) error {
	resp, err := c.Send(Request{
		Type: ReqGroupEnable,
		Data: GroupRequest{Name: name, Host: host},
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GroupDisable disables a tunnel group
func (c *Client) GroupDisable(name string) error {
	resp, err := c.Send(Request{
		Type: ReqGroupDisable,
		Data: GroupRequest{Name: name},
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// IsDaemonRunning checks if the daemon is running by trying to ping it
func IsDaemonRunning() bool {
	client, err := NewClient()
	if err != nil {
		return false
	}
	return client.Ping() == nil
}
