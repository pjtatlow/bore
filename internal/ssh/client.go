package ssh

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pjtatlow/bore/internal/config"
	"golang.org/x/crypto/ssh"
)

// Client wraps an SSH connection with keepalive and reconnection support
type Client struct {
	mu     sync.RWMutex
	client *ssh.Client
	host   config.Host
	cfg    *config.Config

	keepAliveStop chan struct{}
	onDisconnect  func(error)
}

// NewClient creates a new SSH client wrapper
func NewClient(host config.Host, cfg *config.Config) *Client {
	return &Client{
		host: host,
		cfg:  cfg,
	}
}

// Connect establishes the SSH connection
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	authMethods, err := AuthMethods(c.host.IdentityFile)
	if err != nil {
		return fmt.Errorf("failed to get auth methods: %w", err)
	}

	user := c.host.User
	if user == "" {
		user = "root"
	}

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: implement proper host key verification
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", c.host.Hostname, c.host.Port)

	// Handle ProxyJump if configured
	var conn net.Conn
	if c.host.ProxyJump != "" {
		conn, err = c.dialViaProxy(ctx, addr, sshConfig)
	} else {
		conn, err = c.dialDirect(ctx, addr)
	}
	if err != nil {
		return err
	}

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SSH handshake failed: %w", err)
	}

	c.client = ssh.NewClient(sshConn, chans, reqs)

	// Start keepalive
	c.keepAliveStop = make(chan struct{})
	go c.keepAlive()

	return nil
}

// dialDirect connects directly to the target host
func (c *Client) dialDirect(ctx context.Context, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	return conn, nil
}

// dialViaProxy connects through a jump host
func (c *Client) dialViaProxy(ctx context.Context, targetAddr string, sshConfig *ssh.ClientConfig) (net.Conn, error) {
	// Resolve the proxy host
	sshReader, err := config.NewSSHConfigReader()
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH config: %w", err)
	}

	proxyHost := config.ResolveHost(c.host.ProxyJump, config.Host{}, sshReader)
	proxyAddr := fmt.Sprintf("%s:%d", proxyHost.Hostname, proxyHost.Port)

	// Connect to proxy
	proxyConn, err := c.dialDirect(ctx, proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy %s: %w", proxyAddr, err)
	}

	// SSH handshake with proxy
	proxySSHConfig := &ssh.ClientConfig{
		User:            proxyHost.User,
		Auth:            sshConfig.Auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
	if proxySSHConfig.User == "" {
		proxySSHConfig.User = sshConfig.User
	}

	proxySSHConn, proxyChans, proxyReqs, err := ssh.NewClientConn(proxyConn, proxyAddr, proxySSHConfig)
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("SSH handshake with proxy failed: %w", err)
	}

	proxyClient := ssh.NewClient(proxySSHConn, proxyChans, proxyReqs)

	// Dial target through proxy
	conn, err := proxyClient.Dial("tcp", targetAddr)
	if err != nil {
		proxyClient.Close()
		return nil, fmt.Errorf("failed to dial through proxy: %w", err)
	}

	return conn, nil
}

// keepAlive sends periodic keepalive requests
func (c *Client) keepAlive() {
	interval := c.cfg.Defaults.KeepAlive.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.keepAliveStop:
			return
		case <-ticker.C:
			c.mu.RLock()
			client := c.client
			c.mu.RUnlock()

			if client == nil {
				return
			}

			_, _, err := client.SendRequest("keepalive@golang.com", true, nil)
			if err != nil {
				if c.onDisconnect != nil {
					c.onDisconnect(err)
				}
				return
			}
		}
	}
}

// Close closes the SSH connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.keepAliveStop != nil {
		close(c.keepAliveStop)
		c.keepAliveStop = nil
	}

	if c.client != nil {
		err := c.client.Close()
		c.client = nil
		return err
	}

	return nil
}

// SetOnDisconnect sets a callback to be called when the connection is lost
func (c *Client) SetOnDisconnect(fn func(error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onDisconnect = fn
}

// Dial opens a connection to a remote address through the SSH connection
func (c *Client) Dial(network, addr string) (net.Conn, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("SSH client not connected")
	}

	return client.Dial(network, addr)
}

// Listen starts listening on a remote address
func (c *Client) Listen(network, addr string) (net.Listener, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("SSH client not connected")
	}

	return client.Listen(network, addr)
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client != nil
}

// CheckHealth performs an immediate keepalive check with a timeout and returns any error.
// If the check fails, the onDisconnect callback is called.
func (c *Client) CheckHealth(timeout time.Duration) error {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("not connected")
	}

	// Run keepalive with timeout
	errCh := make(chan error, 1)
	go func() {
		_, _, err := client.SendRequest("keepalive@golang.com", true, nil)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err != nil {
			if c.onDisconnect != nil {
				c.onDisconnect(err)
			}
			return err
		}
		return nil
	case <-time.After(timeout):
		err := fmt.Errorf("health check timed out")
		if c.onDisconnect != nil {
			c.onDisconnect(err)
		}
		return err
	}
}
