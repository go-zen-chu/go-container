package runtime

// ExecOptions holds parameters for running a container.
type ExecOptions struct {
	// ImageRootfs is the path to the extracted image rootfs directory.
	ImageRootfs string
	// Command is the command to run inside the container.
	// Defaults to /bin/sh if empty.
	Command []string
	// Interactive enables stdin attachment.
	Interactive bool
	// TTY allocates a pseudo-terminal.
	TTY bool
	// ContainerID is used to identify the container.
	ContainerID string
}

// Runtime is the interface for container execution backends.
type Runtime interface {
	// Name returns the name of the runtime.
	Name() string
	// Exec runs a command inside an isolated environment using the provided rootfs.
	Exec(opts ExecOptions) (pid int, err error)
}
