package image_test

import (
	"os"
	"testing"
	"time"

	"github.com/go-zen-chu/go-container/internal/image"
)

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantName string
		wantTag  string
	}{
		{"alpine:latest", "alpine", "latest"},
		{"alpine", "alpine", "latest"},
		{"ubuntu:22.04", "ubuntu", "22.04"},
		{"myregistry.io/myimage:v1.0", "myregistry.io/myimage", "v1.0"},
		{"myregistry.io:5000/myimage:v1.0", "myregistry.io:5000/myimage", "v1.0"},
		{"myregistry.io/org/image:tag", "myregistry.io/org/image", "tag"},
	}
	for _, tt := range tests {
		name, tag := image.ParseImageRef(tt.ref)
		if name != tt.wantName || tag != tt.wantTag {
			t.Errorf("ParseImageRef(%q) = (%q, %q), want (%q, %q)",
				tt.ref, name, tag, tt.wantName, tt.wantTag)
		}
	}
}

func TestStore(t *testing.T) {
	dir := t.TempDir()
	store, err := image.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Should be empty initially
	imgs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(imgs) != 0 {
		t.Errorf("expected 0 images, got %d", len(imgs))
	}

	// Save metadata
	info := image.ImageInfo{
		Name:      "alpine",
		Tag:       "latest",
		Digest:    "sha256:abc123",
		Size:      1024 * 1024,
		CreatedAt: time.Now().UTC(),
	}
	if err := store.SaveMeta(info); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}

	// Check it exists
	if !store.Exists("alpine", "latest") {
		t.Error("expected image to exist after SaveMeta")
	}

	// Read it back
	got, err := store.GetMeta("alpine", "latest")
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if got.Name != info.Name || got.Tag != info.Tag || got.Digest != info.Digest {
		t.Errorf("GetMeta mismatch: got %+v, want %+v", got, info)
	}

	// List should return one image
	imgs, err = store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(imgs) != 1 {
		t.Errorf("expected 1 image, got %d", len(imgs))
	}

	// Remove it
	if err := store.Remove("alpine", "latest"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if store.Exists("alpine", "latest") {
		t.Error("expected image to not exist after Remove")
	}

	// GetMeta on a non-existent image should return an error
	_, err = store.GetMeta("alpine", "latest")
	if err == nil {
		t.Error("expected error for missing image, got nil")
	}
}

func TestDefaultStoreDir(t *testing.T) {
	dir, err := image.DefaultStoreDir()
	if err != nil {
		t.Fatalf("DefaultStoreDir: %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty default store dir")
	}
	// Should be under the user's home directory
	home, _ := os.UserHomeDir()
	if len(dir) <= len(home) {
		t.Errorf("expected dir %q to be under home %q", dir, home)
	}
}
