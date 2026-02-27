package container_test

import (
	"testing"
	"time"

	"github.com/go-zen-chu/go-container/internal/container"
)

func TestManager(t *testing.T) {
	dir := t.TempDir()
	mgr, err := container.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Initially empty
	containers, err := mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(containers))
	}

	// Save a container
	info := container.ContainerInfo{
		ID:        "test-container-1",
		Image:     "alpine",
		Tag:       "latest",
		Command:   "/bin/sh",
		Status:    container.StatusCreated,
		PID:       0,
		CreatedAt: time.Now().UTC(),
	}
	if err := mgr.Save(info); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Retrieve it
	got, err := mgr.Get("test-container-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != info.ID || got.Image != info.Image || got.Status != info.Status {
		t.Errorf("Get mismatch: got %+v, want %+v", got, info)
	}

	// List should return one container
	containers, err = mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(containers))
	}

	// Update status
	info.Status = container.StatusRunning
	if err := mgr.Save(info); err != nil {
		t.Fatalf("Save (update): %v", err)
	}
	got, err = mgr.Get("test-container-1")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Status != container.StatusRunning {
		t.Errorf("expected status %q, got %q", container.StatusRunning, got.Status)
	}

	// Remove it
	if err := mgr.Remove("test-container-1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_, err = mgr.Get("test-container-1")
	if err == nil {
		t.Error("expected error for missing container, got nil")
	}
}

func TestDefaultStateDir(t *testing.T) {
	dir, err := container.DefaultStateDir()
	if err != nil {
		t.Fatalf("DefaultStateDir: %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty default state dir")
	}
}
