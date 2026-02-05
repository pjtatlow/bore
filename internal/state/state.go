package state

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/pjtatlow/bore/internal/ipc"
)

// TunnelState tracks an active tunnel and its host
type TunnelState struct {
	Name string `json:"name"`
	Host string `json:"host"`
}

// GroupState tracks an active group and its host
type GroupState struct {
	Name string `json:"name"`
	Host string `json:"host"`
}

// State represents the persisted daemon state
type State struct {
	mu            sync.RWMutex
	StartTime     time.Time     `json:"start_time"`
	ActiveTunnels []TunnelState `json:"active_tunnels"`
	ActiveGroups  []GroupState  `json:"active_groups"`
	path          string
}

// NewState creates a new state instance
func NewState() (*State, error) {
	path, err := ipc.StatePath()
	if err != nil {
		return nil, err
	}
	return &State{
		path:          path,
		StartTime:     time.Now(),
		ActiveTunnels: []TunnelState{},
		ActiveGroups:  []GroupState{},
	}, nil
}

// Load reads the state from disk
func (s *State) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Preserve StartTime - it should reflect when this daemon started, not the previous one
	startTime := s.StartTime
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}
	s.StartTime = startTime
	return nil
}

// Save writes the state to disk
func (s *State) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0600)
}

// AddTunnel adds a tunnel to the active list
func (s *State) AddTunnel(name, host string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, t := range s.ActiveTunnels {
		if t.Name == name {
			// Update host if tunnel already exists
			s.ActiveTunnels[i].Host = host
			return
		}
	}
	s.ActiveTunnels = append(s.ActiveTunnels, TunnelState{Name: name, Host: host})
}

// RemoveTunnel removes a tunnel from the active list
func (s *State) RemoveTunnel(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, t := range s.ActiveTunnels {
		if t.Name == name {
			s.ActiveTunnels = append(s.ActiveTunnels[:i], s.ActiveTunnels[i+1:]...)
			return
		}
	}
}

// AddGroup adds a group to the active list
func (s *State) AddGroup(name, host string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, g := range s.ActiveGroups {
		if g.Name == name {
			// Update host if group already exists
			s.ActiveGroups[i].Host = host
			return
		}
	}
	s.ActiveGroups = append(s.ActiveGroups, GroupState{Name: name, Host: host})
}

// RemoveGroup removes a group from the active list
func (s *State) RemoveGroup(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, g := range s.ActiveGroups {
		if g.Name == name {
			s.ActiveGroups = append(s.ActiveGroups[:i], s.ActiveGroups[i+1:]...)
			return
		}
	}
}

// GetActiveTunnels returns a copy of active tunnel states
func (s *State) GetActiveTunnels() []TunnelState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]TunnelState, len(s.ActiveTunnels))
	copy(result, s.ActiveTunnels)
	return result
}

// GetActiveGroups returns a copy of active group states
func (s *State) GetActiveGroups() []GroupState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]GroupState, len(s.ActiveGroups))
	copy(result, s.ActiveGroups)
	return result
}

// Clear removes all state
func (s *State) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ActiveTunnels = []TunnelState{}
	s.ActiveGroups = []GroupState{}
}

// Delete removes the state file
func (s *State) Delete() error {
	return os.Remove(s.path)
}

// Uptime returns the duration since the daemon started
func (s *State) Uptime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.StartTime)
}
