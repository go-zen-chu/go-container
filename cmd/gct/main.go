package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"text/tabwriter"
	"time"

	"github.com/go-zen-chu/go-container/internal/container"
	"github.com/go-zen-chu/go-container/internal/image"
	gctruntime "github.com/go-zen-chu/go-container/internal/runtime"
	"github.com/spf13/cobra"
)

func main() {
	// Handle the Linux runtime child re-exec before cobra takes over.
	// This is invoked internally by the Linux runtime and never by the user directly.
	if len(os.Args) >= 4 && os.Args[1] == "__runtime_child" {
		if runtime.GOOS != "linux" {
			fmt.Fprintln(os.Stderr, "__runtime_child is only supported on Linux")
			os.Exit(1)
		}
		containerID := os.Args[2]
		rootfsDir := os.Args[3]
		command := os.Args[4:]
		if len(command) == 0 {
			command = []string{"/bin/sh"}
		}
		if err := gctruntime.RunChild(containerID, rootfsDir, command); err != nil {
			fmt.Fprintf(os.Stderr, "container child error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gct",
		Short: "gct - a lightweight OCI container manager for Linux and macOS",
		Long: `gct is a lightweight container management tool that supports OCI images.
It runs containers without requiring a VM or virtualization layer.`,
	}

	cmd.AddCommand(
		pullCmd(),
		pushCmd(),
		execCmd(),
		imageCmd(),
		containerCmd(),
	)
	return cmd
}

// storeDir returns the image store directory, exiting on error.
func storeDir() string {
	dir, err := image.DefaultStoreDir()
	if err != nil {
		log.Fatalf("getting image store directory: %v", err)
	}
	return dir
}

// containerStateDir returns the container state directory.
func containerStateDir() string {
	dir, err := container.DefaultStateDir()
	if err != nil {
		log.Fatalf("getting container state directory: %v", err)
	}
	return dir
}

// -----------------------------------------------------------------------
// gct pull <image>
// -----------------------------------------------------------------------

func pullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <image>",
		Short: "Pull an OCI image from a registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			store, err := image.NewStore(storeDir())
			if err != nil {
				return err
			}
			return image.Pull(ref, store)
		},
	}
}

// -----------------------------------------------------------------------
// gct push <image>
// -----------------------------------------------------------------------

func pushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push <image>",
		Short: "Push an OCI image to a registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			store, err := image.NewStore(storeDir())
			if err != nil {
				return err
			}
			return image.Push(ref, store)
		},
	}
}

// -----------------------------------------------------------------------
// gct exec [-it] <image> [command...]
// -----------------------------------------------------------------------

func execCmd() *cobra.Command {
	var interactive bool
	var tty bool

	cmd := &cobra.Command{
		Use:   "exec [flags] <image> [command...]",
		Short: "Run a command in a container created from an OCI image",
		Example: `  gct exec -it alpine:latest /bin/sh
  gct exec ubuntu:22.04 /bin/bash`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			command := args[1:]
			if len(command) == 0 {
				command = []string{"/bin/sh"}
			}

			store, err := image.NewStore(storeDir())
			if err != nil {
				return err
			}

			name, tag := image.ParseImageRef(ref)

			// Auto-pull if not available locally
			if !store.Exists(name, tag) {
				log.Printf("Image %s:%s not found locally, pulling ...", name, tag)
				if err := image.Pull(ref, store); err != nil {
					return fmt.Errorf("pulling image: %w", err)
				}
			}

			stateDir, err := container.DefaultStateDir()
			if err != nil {
				return err
			}
			mgr, err := container.NewManager(stateDir)
			if err != nil {
				return err
			}

			containerID := fmt.Sprintf("%s-%s-%d", name, tag, time.Now().UnixMilli())
			// Sanitize ID for use as a filename / cgroup name
			for _, c := range []string{"/", ":", "."} {
				containerID = replaceAll(containerID, c, "-")
			}

			info := container.ContainerInfo{
				ID:        containerID,
				Image:     name,
				Tag:       tag,
				Command:   fmt.Sprintf("%v", command),
				Status:    container.StatusCreated,
				CreatedAt: time.Now().UTC(),
			}
			if err := mgr.Save(info); err != nil {
				return fmt.Errorf("saving container state: %w", err)
			}

			rt := gctruntime.New()
			log.Printf("Using runtime: %s", rt.Name())

			rootfs := store.RootfsDir(name, tag)
			opts := gctruntime.ExecOptions{
				ImageRootfs: rootfs,
				Command:     command,
				Interactive: interactive,
				TTY:         tty,
				ContainerID: containerID,
			}

			info.Status = container.StatusRunning
			_ = mgr.Save(info)

			pid, err := rt.Exec(opts)
			info.PID = pid
			info.Status = container.StatusStopped
			_ = mgr.Save(info)

			return err
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Keep stdin open")
	cmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a pseudo-TTY")
	return cmd
}

// replaceAll replaces all occurrences of old with new in s.
func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}

// -----------------------------------------------------------------------
// gct image list / gct image rm
// -----------------------------------------------------------------------

func imageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Manage locally stored OCI images",
	}
	cmd.AddCommand(imageListCmd(), imageRmCmd())
	return cmd
}

func imageListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List locally stored OCI images",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := image.NewStore(storeDir())
			if err != nil {
				return err
			}
			images, err := store.List()
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NAME\tTAG\tDIGEST\tSIZE\tCREATED")
			for _, img := range images {
				digest := img.Digest
				if len(digest) > 19 {
					digest = digest[:19] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					img.Name, img.Tag, digest,
					formatSize(img.Size),
					img.CreatedAt.Format(time.RFC3339),
				)
			}
			return w.Flush()
		},
	}
}

func imageRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <image>",
		Short: "Remove a locally stored OCI image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			store, err := image.NewStore(storeDir())
			if err != nil {
				return err
			}
			name, tag := image.ParseImageRef(ref)
			if err := store.Remove(name, tag); err != nil {
				return err
			}
			fmt.Printf("Removed %s:%s\n", name, tag)
			return nil
		},
	}
}

// -----------------------------------------------------------------------
// gct container list / gct container rm
// -----------------------------------------------------------------------

func containerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "container",
		Short: "Manage containers",
	}
	cmd.AddCommand(containerListCmd(), containerRmCmd())
	return cmd
}

func containerListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := container.NewManager(containerStateDir())
			if err != nil {
				return err
			}
			containers, err := mgr.List()
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tIMAGE\tCOMMAND\tSTATUS\tCREATED")
			for _, c := range containers {
				shortID := c.ID
				if len(shortID) > 16 {
					shortID = shortID[:16]
				}
				fmt.Fprintf(w, "%s\t%s:%s\t%s\t%s\t%s\n",
					shortID, c.Image, c.Tag, c.Command, c.Status,
					c.CreatedAt.Format(time.RFC3339),
				)
			}
			return w.Flush()
		},
	}
}

func containerRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <container-id>",
		Short: "Remove a container's state record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := container.NewManager(containerStateDir())
			if err != nil {
				return err
			}
			if err := mgr.Remove(args[0]); err != nil {
				return err
			}
			fmt.Printf("Removed container %s\n", args[0])
			return nil
		},
	}
}

// formatSize returns a human-readable size string.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
