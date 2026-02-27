# go-container

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fgo-zen-chu%2Fgo-container.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fgo-zen-chu%2Fgo-container?ref=badge_shield)

`gct` is a lightweight OCI container management CLI for Linux and macOS, written in Go.
It runs containers without requiring a VM or any virtualization layer.

## Features

- Pull and push OCI images from/to any OCI-compatible registry (Docker Hub, GHCR, ECR, etc.)
- Run containers from OCI images
- List locally stored images and containers
- **Linux**: Full isolation using kernel namespaces (UTS, PID, mount), cgroups v2, and `pivot_root`
- **macOS**: Filesystem isolation via `chroot` (no VM required; kernel namespaces not available on macOS)

## Build

```bash
# Build for the current platform
make build            # outputs ./gct

# Build for Linux (cross-compile from macOS)
make build-linux      # outputs ./gct (linux/amd64)

# Install into GOPATH/bin
make install
```

## Usage

```bash
# Pull an OCI image from a registry
gct pull alpine:latest
gct pull ubuntu:22.04

# Push a locally stored image to a registry
gct push myregistry.io/myimage:v1.0

# Run a command interactively inside a container
gct exec -it alpine:latest /bin/sh
gct exec -it ubuntu:22.04 /bin/bash

# Run a one-off command
gct exec alpine:latest /bin/echo hello

# List locally stored images
gct image list

# Remove a locally stored image
gct image rm alpine:latest

# List all containers (running and stopped)
gct container list

# Remove a container state record
gct container rm <container-id>
```

## How it works

### Linux

On Linux, `gct exec` re-invokes itself inside new Linux namespaces
(`CLONE_NEWUTS | CLONE_NEWPID | CLONE_NEWNS`) using `/proc/self/exe`.
Inside the new namespaces it:
1. Creates a cgroups v2 hierarchy to limit memory usage.
2. Bind-mounts the image rootfs and calls `pivot_root` to jail the filesystem.
3. Mounts a fresh `/proc` inside the container.
4. `execve`s the requested command.

### macOS

On macOS, Linux kernel namespaces are not available. `gct exec` uses the system
`chroot` command to provide filesystem isolation. Running as root is required for
`chroot` on macOS.

## Local image storage

Images are stored in `~/.gct/images/<name>_<tag>/`:

```
~/.gct/
  images/
    alpine_latest/
      meta.json      # image metadata (name, tag, digest, size, created_at)
      rootfs/        # extracted image filesystem
  containers/
    <container-id>.json   # container state (id, image, status, pid, ...)
```

## FAQ

### Building for Linux on macOS

The cgroups library requires Linux headers. Cross-compile with:

```bash
GOARCH=amd64 GOOS=linux go build -o gct ./cmd/gct
```

### `operation not permitted` when running `gct exec`

Container isolation (namespaces, `pivot_root`, cgroups) requires root privileges.
Run with `sudo` or as root.

## License

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fgo-zen-chu%2Fgo-container.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fgo-zen-chu%2Fgo-container?ref=badge_large)
