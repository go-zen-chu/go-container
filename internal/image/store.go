package image

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ImageInfo holds metadata about a locally stored OCI image.
type ImageInfo struct {
	Name      string    `json:"name"`
	Tag       string    `json:"tag"`
	Digest    string    `json:"digest"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// ID returns a unique identifier for the image (name:tag).
func (i ImageInfo) ID() string {
	return i.Name + ":" + i.Tag
}

// Store manages locally stored OCI images.
type Store struct {
	rootDir string
}

// NewStore creates a new image store at the given root directory.
func NewStore(rootDir string) (*Store, error) {
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("creating image store directory: %w", err)
	}
	return &Store{rootDir: rootDir}, nil
}

// DefaultStoreDir returns the default image store directory.
func DefaultStoreDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".gct", "images"), nil
}

// ImageDir returns the directory for a specific image.
// The name is sanitized so it is safe to use as a directory name.
func (s *Store) ImageDir(name, tag string) string {
	safe := sanitizeDirName(name) + "_" + sanitizeDirName(tag)
	return filepath.Join(s.rootDir, safe)
}

// sanitizeDirName replaces characters that are unsafe in directory names
// with underscores. This covers registry host:port, slashes, colons, etc.
func sanitizeDirName(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '.' {
			result[i] = c
		} else {
			result[i] = '_'
		}
	}
	return string(result)
}

// RootfsDir returns the rootfs directory for an image.
func (s *Store) RootfsDir(name, tag string) string {
	return filepath.Join(s.ImageDir(name, tag), "rootfs")
}

// MetaPath returns the path to the image metadata file.
func (s *Store) MetaPath(name, tag string) string {
	return filepath.Join(s.ImageDir(name, tag), "meta.json")
}

// SaveMeta writes image metadata to disk.
func (s *Store) SaveMeta(info ImageInfo) error {
	dir := s.ImageDir(info.Name, info.Tag)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating image directory: %w", err)
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling image metadata: %w", err)
	}
	if err := os.WriteFile(s.MetaPath(info.Name, info.Tag), data, 0644); err != nil {
		return fmt.Errorf("writing image metadata: %w", err)
	}
	return nil
}

// GetMeta reads image metadata from disk.
func (s *Store) GetMeta(name, tag string) (*ImageInfo, error) {
	data, err := os.ReadFile(s.MetaPath(name, tag))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("image %s:%s not found locally", name, tag)
		}
		return nil, fmt.Errorf("reading image metadata: %w", err)
	}
	var info ImageInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("unmarshaling image metadata: %w", err)
	}
	return &info, nil
}

// List returns all locally stored images.
func (s *Store) List() ([]ImageInfo, error) {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading image store: %w", err)
	}
	var images []ImageInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(s.rootDir, entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var info ImageInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}
		images = append(images, info)
	}
	return images, nil
}

// Exists returns true if the image is stored locally.
func (s *Store) Exists(name, tag string) bool {
	_, err := os.Stat(s.MetaPath(name, tag))
	return err == nil
}

// Remove deletes a locally stored image.
func (s *Store) Remove(name, tag string) error {
	dir := s.ImageDir(name, tag)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing image %s:%s: %w", name, tag, err)
	}
	return nil
}
