package image

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// ParseImageRef parses an image reference into name and tag.
// e.g. "alpine:latest" -> ("alpine", "latest")
//      "ubuntu"        -> ("ubuntu", "latest")
//      "myregistry.io/myimage:v1.0" -> ("myregistry.io/myimage", "v1.0")
func ParseImageRef(ref string) (name, tag string) {
	// Find the last colon that is after the last slash (to avoid parsing registry host ports)
	lastSlash := strings.LastIndex(ref, "/")
	searchIn := ref[lastSlash+1:]
	if colonIdx := strings.LastIndex(searchIn, ":"); colonIdx >= 0 {
		tag = searchIn[colonIdx+1:]
		name = ref[:lastSlash+1+colonIdx]
	} else {
		name = ref
		tag = "latest"
	}
	return name, tag
}

// Pull fetches an OCI image from a registry and stores it in the local store.
func Pull(ref string, store *Store, options ...crane.Option) error {
	name, tag := ParseImageRef(ref)
	log.Printf("Pulling %s:%s ...", name, tag)

	img, err := crane.Pull(ref, options...)
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", ref, err)
	}

	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("getting image digest: %w", err)
	}

	size, err := imageSize(img)
	if err != nil {
		return fmt.Errorf("getting image size: %w", err)
	}

	rootfsDir := store.RootfsDir(name, tag)
	if err := os.MkdirAll(rootfsDir, 0755); err != nil {
		return fmt.Errorf("creating rootfs directory: %w", err)
	}

	log.Printf("Extracting layers to %s ...", rootfsDir)
	if err := extractImage(img, rootfsDir); err != nil {
		return fmt.Errorf("extracting image layers: %w", err)
	}

	info := ImageInfo{
		Name:      name,
		Tag:       tag,
		Digest:    digest.String(),
		Size:      size,
		CreatedAt: time.Now().UTC(),
	}
	if err := store.SaveMeta(info); err != nil {
		return fmt.Errorf("saving image metadata: %w", err)
	}

	log.Printf("Successfully pulled %s:%s (%s)", name, tag, digest)
	return nil
}

// imageSize returns the total compressed size of the image layers.
func imageSize(img v1.Image) (int64, error) {
	manifest, err := img.Manifest()
	if err != nil {
		return 0, err
	}
	var total int64
	for _, layer := range manifest.Layers {
		total += layer.Size
	}
	return total, nil
}

// extractImage unpacks all layers of the image into destDir.
func extractImage(img v1.Image, destDir string) error {
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("getting layers: %w", err)
	}
	for i, layer := range layers {
		log.Printf("Extracting layer %d/%d ...", i+1, len(layers))
		rc, err := layer.Uncompressed()
		if err != nil {
			return fmt.Errorf("uncompressing layer %d: %w", i, err)
		}
		if err := extractTar(rc, destDir); err != nil {
			rc.Close()
			return fmt.Errorf("extracting layer %d: %w", i, err)
		}
		rc.Close()
	}
	return nil
}

// extractTar extracts a tar archive into destDir.
// It handles whiteout files (Docker layer deletions).
func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// Clean the path to prevent path traversal
		cleanName := filepath.Clean(hdr.Name)

		// Build the target path and ensure it stays within destDir
		target := filepath.Join(destDir, cleanName)
		cleanTarget := filepath.Clean(target)
		destPath := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(cleanTarget, destPath) {
			// Skip entries that would escape the destination directory
			continue
		}
		target = cleanTarget
		// Handle whiteout files (layer deletions)
		base := filepath.Base(cleanName)
		dir := filepath.Dir(cleanName)
		if strings.HasPrefix(base, ".wh.") {
			// Whiteout: delete the corresponding file/dir
			if base == ".wh..wh..opq" {
				// Opaque whiteout: delete all contents of dir
				fullDir := filepath.Join(destDir, dir)
				entries, _ := os.ReadDir(fullDir)
				for _, e := range entries {
					os.RemoveAll(filepath.Join(fullDir, e.Name()))
				}
			} else {
				// Normal whiteout
				deleteName := strings.TrimPrefix(base, ".wh.")
				os.RemoveAll(filepath.Join(destDir, dir, deleteName))
			}
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)|0111); err != nil {
				return fmt.Errorf("creating directory %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("creating parent dir for %s: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("creating file %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("writing file %s: %w", target, err)
			}
			f.Close()
		case tar.TypeSymlink:
			// Absolute symlink targets (e.g. "/bin/busybox") are valid in container
			// images and will resolve correctly inside the container after chroot /
			// pivot_root. We create them as-is. Only relative symlinks that resolve
			// outside destDir are rejected to prevent host path traversal.
			linkname := hdr.Linkname
			if !filepath.IsAbs(linkname) {
				// Resolve the relative symlink from the directory that holds it
				// and verify the resolved path stays within destDir.
				resolvedTarget := filepath.Clean(filepath.Join(filepath.Dir(target), linkname))
				if !strings.HasPrefix(resolvedTarget, destDir+string(os.PathSeparator)) && resolvedTarget != destDir {
					log.Printf("Warning: skipping symlink %s -> %s: resolved target outside destDir", target, linkname)
					continue
				}
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("creating parent dir for symlink %s: %w", target, err)
			}
			// Remove existing file before creating symlink
			os.Remove(target)
			if err := os.Symlink(linkname, target); err != nil {
				// Non-fatal: symlinks may fail in some cases
				log.Printf("Warning: creating symlink %s -> %s: %v", target, linkname, err)
			}
		case tar.TypeLink:
			linkTarget := filepath.Join(destDir, filepath.Clean(hdr.Linkname))
			// Validate that the hard link target stays within destDir to prevent path traversal
			if !strings.HasPrefix(linkTarget, destDir+string(os.PathSeparator)) && linkTarget != destDir {
				log.Printf("Warning: skipping hard link %s -> %s: target outside destDir", target, hdr.Linkname)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("creating parent dir for hard link %s: %w", target, err)
			}
			os.Remove(target)
			if err := os.Link(linkTarget, target); err != nil {
				log.Printf("Warning: creating hard link %s -> %s: %v", target, linkTarget, err)
			}
		}
	}
	return nil
}
