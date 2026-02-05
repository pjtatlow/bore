package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/pjtatlow/bore/internal/ipc"
)

// Server handles IPC requests from clients
type Server struct {
	mu       sync.RWMutex
	listener net.Listener
	handler  RequestHandler
	ctx      context.Context
	cancel   context.CancelFunc
}

// RequestHandler processes IPC requests
type RequestHandler interface {
	HandleRequest(req ipc.Request) ipc.Response
}

// NewServer creates a new IPC server
func NewServer(handler RequestHandler) (*Server, error) {
	return &Server{
		handler: handler,
	}, nil
}

// Start begins listening for client connections
func (s *Server) Start(ctx context.Context) error {
	socketPath, err := ipc.SocketPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(socketPath), 0700); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Remove existing socket file
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}

	// Set socket permissions
	if err := os.Chmod(socketPath, 0600); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	go s.acceptLoop()

	return nil
}

// acceptLoop accepts and handles client connections
func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				continue
			}
		}

		go s.handleConnection(conn)
	}
}

// handleConnection processes a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req ipc.Request
	if err := decoder.Decode(&req); err != nil {
		encoder.Encode(ipc.Response{
			Success: false,
			Error:   fmt.Sprintf("failed to decode request: %v", err),
		})
		return
	}

	resp := s.handler.HandleRequest(req)
	encoder.Encode(resp)
}

// Stop stops the server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	if s.listener != nil {
		s.listener.Close()
	}

	// Clean up socket file
	socketPath, err := ipc.SocketPath()
	if err == nil {
		os.Remove(socketPath)
	}

	return nil
}
