//go:build darwin

package runtime

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

// DarwinRuntime implements a best-effort container execution on macOS.
// macOS does not provide Linux kernel namespaces or cgroups, so isolation
// is limited to a chroot jail (requires root) around the image rootfs.
type DarwinRuntime struct{}

// New returns the macOS container runtime.
func New() Runtime {
	return &DarwinRuntime{}
}

func (r *DarwinRuntime) Name() string {
	return "darwin"
}

// Exec runs a command inside a chroot using the provided rootfs.
// Note: full namespace isolation is not available on macOS.
func (r *DarwinRuntime) Exec(opts ExecOptions) (int, error) {
	command := opts.Command
	if len(command) == 0 {
		command = []string{"/bin/sh"}
	}

	log.Printf("Note: macOS runtime provides filesystem isolation via chroot only (no kernel namespaces)")
	log.Printf("Running %v in rootfs %s ...", command, opts.ImageRootfs)

	cmd := exec.Command("chroot", append([]string{opts.ImageRootfs}, command...)...)
	if opts.Interactive || opts.TTY {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("starting container process (chroot): %w", err)
	}

	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		return pid, fmt.Errorf("container exited with error: %w", err)
	}
	return pid, nil
}

// RunChild is a no-op on macOS (only used by the Linux runtime for re-exec).
func RunChild(containerID, rootfsDir string, command []string) error {
	return fmt.Errorf("__runtime_child re-exec is not supported on macOS")
}
