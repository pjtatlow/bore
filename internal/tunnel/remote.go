package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/pjtatlow/bore/internal/config"
)

// RemoteTunnel implements remote port forwarding (-R)
// It listens on the remote SSH server and forwards connections back locally
type RemoteTunnel struct {
	*baseTunnel
	sshClient SSHListener
	listener  net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// SSHListener defines the interface for SSH listening operations
type SSHListener interface {
	Listen(network, addr string) (net.Listener, error)
}

// NewRemoteTunnel creates a new remote forwarding tunnel
func NewRemoteTunnel(name string, cfg config.Tunnel, client SSHListener) *RemoteTunnel {
	return &RemoteTunnel{
		baseTunnel: newBaseTunnel(name, cfg),
		sshClient:  client,
	}
}

// Start begins listening on the remote server and forwarding connections locally
func (t *RemoteTunnel) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.SetStatus(StatusConnecting, nil)

	// Listen on the remote side via SSH
	remoteAddr := fmt.Sprintf("0.0.0.0:%d", t.config.RemotePort)

	listener, err := t.sshClient.Listen("tcp", remoteAddr)
	if err != nil {
		t.SetStatus(StatusError, err)
		return fmt.Errorf("failed to listen on remote %s: %w", remoteAddr, err)
	}
	t.listener = listener

	t.SetStatus(StatusConnected, nil)

	t.wg.Add(1)
	go t.acceptLoop()

	return nil
}

// acceptLoop accepts incoming connections from the remote side
func (t *RemoteTunnel) acceptLoop() {
	defer t.wg.Done()

	for {
		remoteConn, err := t.listener.Accept()
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
		go t.handleConnection(remoteConn)
	}
}

// handleConnection handles a single forwarded connection
func (t *RemoteTunnel) handleConnection(remoteConn net.Conn) {
	defer t.wg.Done()
	defer remoteConn.Close()

	localAddr := fmt.Sprintf("%s:%d", t.config.LocalHost, t.config.LocalPort)

	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		return
	}
	defer localConn.Close()

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	// Remote -> Local
	go func() {
		defer wg.Done()
		n, _ := io.Copy(localConn, remoteConn)
		t.stats.AddReceived(n)
	}()

	// Local -> Remote
	go func() {
		defer wg.Done()
		n, _ := io.Copy(remoteConn, localConn)
		t.stats.AddSent(n)
	}()

	wg.Wait()
}

// Stop stops the tunnel
func (t *RemoteTunnel) Stop() error {
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
