package image

import (
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Push pushes an OCI image from the local store to a remote registry.
func Push(ref string, store *Store, options ...crane.Option) error {
	imgName, tag := ParseImageRef(ref)
	log.Printf("Pushing %s:%s ...", imgName, tag)

	info, err := store.GetMeta(imgName, tag)
	if err != nil {
		return fmt.Errorf("image not found locally: %w", err)
	}

	// Load the image from the stored tarball path if it exists,
	// otherwise report that push requires pulling first.
	tarPath := store.ImageDir(info.Name, info.Tag) + "/image.tar"

	tag_, err := name.NewTag(ref)
	if err != nil {
		return fmt.Errorf("parsing image reference %s: %w", ref, err)
	}

	img, err := tarball.ImageFromPath(tarPath, &tag_)
	if err != nil {
		return fmt.Errorf("loading image from local store (image.tar not found, re-pull may be required): %w", err)
	}

	log.Printf("Uploading %s:%s to registry ...", imgName, tag)
	if err := crane.Push(img, ref, options...); err != nil {
		return fmt.Errorf("pushing image %s: %w", ref, err)
	}

	log.Printf("Successfully pushed %s:%s", imgName, tag)
	return nil
}
