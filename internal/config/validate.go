package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "\n")
}

// Validate checks the configuration for errors
func (c *Config) Validate() error {
	var errs ValidationErrors

	// Validate defaults
	if c.Defaults.Reconnect.Multiplier <= 0 {
		errs = append(errs, ValidationError{
			Field:   "defaults.reconnect.multiplier",
			Message: "must be greater than 0",
		})
	}
	if c.Defaults.Reconnect.InitialBackoff < 0 {
		errs = append(errs, ValidationError{
			Field:   "defaults.reconnect.initial_backoff",
			Message: "must be non-negative",
		})
	}
	if c.Defaults.Reconnect.MaxBackoff < c.Defaults.Reconnect.InitialBackoff {
		errs = append(errs, ValidationError{
			Field:   "defaults.reconnect.max_backoff",
			Message: "must be greater than or equal to initial_backoff",
		})
	}
	if c.Defaults.KeepAlive.Interval < 0 {
		errs = append(errs, ValidationError{
			Field:   "defaults.keep_alive.interval",
			Message: "must be non-negative",
		})
	}

	// Validate tunnels
	for name, tunnel := range c.Tunnels {
		errs = append(errs, c.validateTunnel(name, tunnel)...)
	}

	// Check for duplicate local ports across tunnels
	portToTunnel := make(map[int]string)
	for name, tunnel := range c.Tunnels {
		if existingName, exists := portToTunnel[tunnel.LocalPort]; exists {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("tunnels.%s.local_port", name),
				Message: fmt.Sprintf("port %d conflicts with tunnel '%s'", tunnel.LocalPort, existingName),
			})
		} else {
			portToTunnel[tunnel.LocalPort] = name
		}
	}

	// Validate groups
	for name, group := range c.Groups {
		errs = append(errs, c.validateGroup(name, group)...)
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (c *Config) validateTunnel(name string, t Tunnel) ValidationErrors {
	var errs ValidationErrors
	prefix := fmt.Sprintf("tunnels.%s", name)

	if t.Type != TunnelTypeLocal && t.Type != TunnelTypeRemote {
		errs = append(errs, ValidationError{
			Field:   prefix + ".type",
			Message: fmt.Sprintf("must be 'local' or 'remote', got '%s'", t.Type),
		})
	}

	// Host field in tunnel config is now optional - host is specified at runtime

	if t.LocalPort <= 0 || t.LocalPort > 65535 {
		errs = append(errs, ValidationError{
			Field:   prefix + ".local_port",
			Message: "must be between 1 and 65535",
		})
	}

	if t.RemotePort <= 0 || t.RemotePort > 65535 {
		errs = append(errs, ValidationError{
			Field:   prefix + ".remote_port",
			Message: "must be between 1 and 65535",
		})
	}

	return errs
}

func (c *Config) validateGroup(name string, g Group) ValidationErrors {
	var errs ValidationErrors
	prefix := fmt.Sprintf("groups.%s", name)

	if len(g.Tunnels) == 0 {
		errs = append(errs, ValidationError{
			Field:   prefix + ".tunnels",
			Message: "must contain at least one tunnel",
		})
	}

	for i, tunnelName := range g.Tunnels {
		if _, ok := c.Tunnels[tunnelName]; !ok {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("%s.tunnels[%d]", prefix, i),
				Message: fmt.Sprintf("references unknown tunnel '%s'", tunnelName),
			})
		}
	}

	return errs
}

// CheckPortConflicts checks for port conflicts between active tunnels and new tunnels
func CheckPortConflicts(activeTunnels map[string]Tunnel, newTunnels map[string]Tunnel) error {
	// Build a map of local ports in use
	usedPorts := make(map[int]string)
	for name, t := range activeTunnels {
		usedPorts[t.LocalPort] = name
	}

	// Check new tunnels for conflicts
	for name, t := range newTunnels {
		if existingName, ok := usedPorts[t.LocalPort]; ok {
			return fmt.Errorf("port conflict: %d already used by tunnel '%s', cannot enable '%s'",
				t.LocalPort, existingName, name)
		}
	}

	return nil
}
