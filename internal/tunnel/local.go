package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/pjtatlow/bore/internal/config"
)

// LocalTunnel implements local port forwarding (-L)
// It listens locally and forwards connections to a remote host via SSH
type LocalTunnel struct {
	*baseTunnel
	sshClient SSHClient
	listener  net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// SSHClient defines the interface for SSH client operations
type SSHClient interface {
	Dial(network, addr string) (net.Conn, error)
}

// NewLocalTunnel creates a new local forwarding tunnel
func NewLocalTunnel(name string, cfg config.Tunnel, client SSHClient) *LocalTunnel {
	return &LocalTunnel{
		baseTunnel: newBaseTunnel(name, cfg),
		sshClient:  client,
	}
}

// Start begins listening for local connections and forwarding them
func (t *LocalTunnel) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.SetStatus(StatusConnecting, nil)

	localAddr := fmt.Sprintf("%s:%d", t.config.LocalHost, t.config.LocalPort)

	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		t.SetStatus(StatusError, err)
		return fmt.Errorf("failed to listen on %s: %w", localAddr, err)
	}
	t.listener = listener

	t.SetStatus(StatusConnected, nil)

	t.wg.Add(1)
	go t.acceptLoop()

	return nil
}

// acceptLoop accepts incoming connections
func (t *LocalTunnel) acceptLoop() {
	defer t.wg.Done()

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.ctx.Done():
				return
			default:
				continue
			}
		}

		t.stats.IncrementConnections()
		t.wg.Add(1)
		go t.handleConnection(conn)
	}
}

// handleConnection handles a single forwarded connection
func (t *LocalTunnel) handleConnection(localConn net.Conn) {
	defer t.wg.Done()
	defer localConn.Close()

	remoteAddr := fmt.Sprintf("%s:%d", t.config.RemoteHost, t.config.RemotePort)

	remoteConn, err := t.sshClient.Dial("tcp", remoteAddr)
	if err != nil {
		return
	}
	defer remoteConn.Close()

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	// Local -> Remote
	go func() {
		defer wg.Done()
		n, _ := io.Copy(remoteConn, localConn)
		t.stats.AddSent(n)
	}()

	// Remote -> Local
	go func() {
		defer wg.Done()
		n, _ := io.Copy(localConn, remoteConn)
		t.stats.AddReceived(n)
	}()

	wg.Wait()
}

// Stop stops the tunnel
func (t *LocalTunnel) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}

	if t.listener != nil {
		t.listener.Close()
	}

	t.wg.Wait()
	t.SetStatus(StatusStopped, nil)
	return nil
}
