package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Status represents the lifecycle state of a container.
type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusCreated Status = "created"
)

// ContainerInfo holds metadata about a container instance.
type ContainerInfo struct {
	ID        string    `json:"id"`
	Image     string    `json:"image"`
	Tag       string    `json:"tag"`
	Command   string    `json:"command"`
	Status    Status    `json:"status"`
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}

// Manager manages container state on disk.
type Manager struct {
	stateDir string
}

// NewManager creates a new container manager at the given state directory.
func NewManager(stateDir string) (*Manager, error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("creating container state directory: %w", err)
	}
	return &Manager{stateDir: stateDir}, nil
}

// DefaultStateDir returns the default container state directory.
func DefaultStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".gct", "containers"), nil
}

// statePath returns the path to a container's state file.
func (m *Manager) statePath(id string) string {
	return filepath.Join(m.stateDir, id+".json")
}

// Save persists a container's state to disk.
func (m *Manager) Save(info ContainerInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling container state: %w", err)
	}
	if err := os.WriteFile(m.statePath(info.ID), data, 0644); err != nil {
		return fmt.Errorf("writing container state: %w", err)
	}
	return nil
}

// Get retrieves a container's state from disk.
func (m *Manager) Get(id string) (*ContainerInfo, error) {
	data, err := os.ReadFile(m.statePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("container %s not found", id)
		}
		return nil, fmt.Errorf("reading container state: %w", err)
	}
	var info ContainerInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("unmarshaling container state: %w", err)
	}
	return &info, nil
}

// List returns all known containers.
func (m *Manager) List() ([]ContainerInfo, error) {
	entries, err := os.ReadDir(m.stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading container state dir: %w", err)
	}
	var containers []ContainerInfo
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.stateDir, entry.Name()))
		if err != nil {
			continue
		}
		var info ContainerInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}
		// Update status if the process is no longer running
		if info.Status == StatusRunning && info.PID > 0 {
			if !isProcessRunning(info.PID) {
				info.Status = StatusStopped
				_ = m.Save(info)
			}
		}
		containers = append(containers, info)
	}
	return containers, nil
}

// Remove deletes a container's state file.
func (m *Manager) Remove(id string) error {
	if err := os.Remove(m.statePath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing container state: %w", err)
	}
	return nil
}

// isProcessRunning checks if a process with the given PID is alive.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; send signal 0 to check existence.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
