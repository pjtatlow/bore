package ipc

import "github.com/pjtatlow/bore/internal/tunnel"

// Request represents a client request to the daemon
type Request struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// Response represents a daemon response to the client
type Response struct {
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// Request types
const (
	ReqStatus       = "status"
	ReqStop         = "stop"
	ReqTunnelUp     = "tunnel_up"
	ReqTunnelDown   = "tunnel_down"
	ReqGroupEnable  = "group_enable"
	ReqGroupDisable = "group_disable"
	ReqReloadConfig = "reload_config"
	ReqPing         = "ping"
)

// StatusResponse contains daemon and tunnel status
type StatusResponse struct {
	Running  bool               `json:"running"`
	PID      int                `json:"pid"`
	Uptime   string             `json:"uptime"`
	Tunnels  []TunnelStatus     `json:"tunnels"`
	Groups   []GroupStatus      `json:"groups"`
	Network  NetworkStatusInfo  `json:"network"`
}

// TunnelStatus contains status info for a single tunnel
type TunnelStatus struct {
	Name           string              `json:"name"`
	Type           string              `json:"type"`
	Host           string              `json:"host"`
	LocalPort      int                 `json:"local_port"`
	RemoteHost     string              `json:"remote_host"`
	RemotePort     int                 `json:"remote_port"`
	Status         tunnel.Status       `json:"status"`
	Error          string              `json:"error,omitempty"`
	BytesSent      int64               `json:"bytes_sent"`
	BytesReceived  int64               `json:"bytes_received"`
	Connections    int64               `json:"connections"`
	ReconnectCount int                 `json:"reconnect_count"`
	Uptime         string              `json:"uptime,omitempty"`
}

// GroupStatus contains status info for a tunnel group
type GroupStatus struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
	Tunnels     []string `json:"tunnels"`
}

// NetworkStatusInfo contains network monitoring status
type NetworkStatusInfo struct {
	Status string `json:"status"`
}

// TunnelRequest is used for tunnel up/down requests
type TunnelRequest struct {
	Name string `json:"name"`
}

// GroupRequest is used for group enable/disable requests
type GroupRequest struct {
	Name string `json:"name"`
}
