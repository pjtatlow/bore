package state

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/pjtatlow/bore/internal/ipc"
)

// State represents the persisted daemon state
type State struct {
	mu            sync.RWMutex
	StartTime     time.Time          `json:"start_time"`
	ActiveTunnels []string           `json:"active_tunnels"`
	ActiveGroups  []string           `json:"active_groups"`
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
		ActiveTunnels: []string{},
		ActiveGroups:  []string{},
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

	return json.Unmarshal(data, s)
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
func (s *State) AddTunnel(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.ActiveTunnels {
		if t == name {
			return
		}
	}
	s.ActiveTunnels = append(s.ActiveTunnels, name)
}

// RemoveTunnel removes a tunnel from the active list
func (s *State) RemoveTunnel(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, t := range s.ActiveTunnels {
		if t == name {
			s.ActiveTunnels = append(s.ActiveTunnels[:i], s.ActiveTunnels[i+1:]...)
			return
		}
	}
}

// AddGroup adds a group to the active list
func (s *State) AddGroup(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, g := range s.ActiveGroups {
		if g == name {
			return
		}
	}
	s.ActiveGroups = append(s.ActiveGroups, name)
}

// RemoveGroup removes a group from the active list
func (s *State) RemoveGroup(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, g := range s.ActiveGroups {
		if g == name {
			s.ActiveGroups = append(s.ActiveGroups[:i], s.ActiveGroups[i+1:]...)
			return
		}
	}
}

// GetActiveTunnels returns a copy of active tunnel names
func (s *State) GetActiveTunnels() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, len(s.ActiveTunnels))
	copy(result, s.ActiveTunnels)
	return result
}

// GetActiveGroups returns a copy of active group names
func (s *State) GetActiveGroups() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, len(s.ActiveGroups))
	copy(result, s.ActiveGroups)
	return result
}

// Clear removes all state
func (s *State) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ActiveTunnels = []string{}
	s.ActiveGroups = []string{}
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
