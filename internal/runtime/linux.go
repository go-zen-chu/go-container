//go:build linux

package runtime

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	cgroupsv2 "github.com/containerd/cgroups/v2"
)

// LinuxRuntime implements container execution on Linux using namespaces,
// cgroups v2, and pivot_root for filesystem isolation.
type LinuxRuntime struct{}

// New returns the Linux container runtime.
func New() Runtime {
	return &LinuxRuntime{}
}

func (r *LinuxRuntime) Name() string {
	return "linux"
}

// Exec forks a child process that sets up the container environment.
// It re-invokes the current executable with the "__runtime_child" argument
// to perform setup inside the new namespaces.
func (r *LinuxRuntime) Exec(opts ExecOptions) (int, error) {
	command := opts.Command
	if len(command) == 0 {
		command = []string{"/bin/sh"}
	}

	// Build args for the child: __runtime_child <rootfs> <cmd> [args...]
	childArgs := append([]string{"__runtime_child", opts.ContainerID, opts.ImageRootfs}, command...)
	cmd := exec.Command("/proc/self/exe", childArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	if opts.Interactive || opts.TTY {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("starting container process: %w", err)
	}

	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		return pid, fmt.Errorf("container exited with error: %w", err)
	}
	return pid, nil
}

// cgroupV2Root is the standard cgroups v2 unified hierarchy mount point.
const cgroupV2Root = "/sys/fs/cgroup"

// RunChild is called when the process is re-invoked as "__runtime_child".
// It sets up cgroups, pivot_root, mounts, and then execs the user command.
func RunChild(containerID, rootfsDir string, command []string) error {
	log.Printf("child: setting up container %s", containerID)

	cgroupName := "/gct-" + containerID
	minMem := int64(4 * 1024 * 1024)   // 4 MiB minimum
	maxMem := int64(512 * 1024 * 1024) // 512 MiB maximum
	res := cgroupsv2.Resources{
		Memory: &cgroupsv2.Memory{
			Min: &minMem,
			Max: &maxMem,
		},
	}
	mgr, err := cgroupsv2.NewManager(cgroupV2Root, cgroupName, &res)
	if err != nil {
		// Non-fatal: cgroup setup may fail without root privileges
		log.Printf("Warning: creating cgroup %s: %v (continuing without memory limits)", cgroupName, err)
	} else {
		defer mgr.Delete()
		log.Printf("cgroups v2 %s created successfully", cgroupName)
	}

	// Set up pivot_root
	putold := rootfsDir + "/putold"
	if err := os.MkdirAll(putold, 0755); err != nil {
		return fmt.Errorf("creating putold directory: %w", err)
	}
	if err := syscall.Mount(rootfsDir, rootfsDir, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mounting rootfs: %w", err)
	}
	if err := syscall.PivotRoot(rootfsDir, putold); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir /: %w", err)
	}
	if err := syscall.Unmount("/putold", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmounting putold: %w", err)
	}
	if err := os.Remove("/putold"); err != nil {
		log.Printf("Warning: removing /putold: %v", err)
	}

	// Mount /proc inside the container
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		log.Printf("Warning: mounting /proc: %v (continuing without /proc)", err)
	}

	// Exec the user command (replaces current process)
	path, err := exec.LookPath(command[0])
	if err != nil {
		path = command[0]
	}
	return syscall.Exec(path, command, os.Environ())
}
