package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/containerd/cgroups"
	cgroupsv2 "github.com/containerd/cgroups/v2"
)

func main() {
	if err := checkEnv(); err != nil {
		panic(err)
	}
	if len(os.Args) == 1 {
		panic("usage: ./go-container run <command for running container>")
	}
	switch os.Args[1] {
	case "run":
		if err := parent(); err != nil {
			panic(err)
		}
	case "child":
		if err := child(); err != nil {
			panic(err)
		}
	default:
		panic(fmt.Sprintf("not supported command: %s", os.Args[1]))
	}
	log.Println("finishing container...")
}

func checkEnv() error {
	if runtime.GOOS == "linux" {
		log.Println("recognizing as linux")
	} else {
		return errors.New("only linux is supported")
	}
	if cgroups.Mode() == cgroups.Unified {
		log.Println("running linux with cgroups v2")
	} else {
		return errors.New("cgroups v1 is not supported")
	}
	return nil
}

func profile() error {
	log.Println("[PROFILE] current process status:")
	fmt.Printf("[PROFILE PIDs] pid: %d, parent pid: %d, uid: %d, gid: %d\n", os.Getpid(), os.Getppid(), os.Getuid(), os.Getgid())
	if dir, err := os.Getwd(); err != nil {
		return err
	} else {
		fmt.Printf("[PROFILE DIRS] current dir and files: %s\n", dir)
		if files, err := ioutil.ReadDir(dir); err != nil {
			return err
		} else {
			for _, file := range files {
				fmt.Printf(" |- %s\n", file.Name())
			}
		}
	}
	if _, err := os.Stat("/proc/self"); err == nil {
		if mounts, err := ioutil.ReadFile("/proc/self/mounts"); err != nil {
			return fmt.Errorf("read mounts: %w", err)
		} else {
			mlines := strings.Split(string(mounts), "\n")
			fmt.Println("[PROFILE MOUNTS] mount info:")
			for _, m := range mlines {
				fmt.Printf(" |- %s\n", m)
			}
		}
		if cgroups, err := ioutil.ReadFile("/proc/self/cgroup"); err != nil {
			return err
		} else {
			cglines := strings.Split(string(cgroups), "\n")
			fmt.Println("[PROFILE CGROUPS] cgroups:")
			for _, cg := range cglines {
				fmt.Printf(" |- %s\n", cg)
			}
		}
	} else {
		fmt.Println("cannot find /proc/self from current path")
	}
	return nil
}

// parent function update current process as container
func parent() error {
	log.Println("===========================================")
	log.Println("starting parent process")
	log.Println("===========================================")
	if err := profile(); err != nil {
		return err
	}
	// TIPS: /proc/self/exe is a special file containing an in-memory image of the current executable in Linux
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	// parameters for making child process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// create process with new UTS, new PID (=1), new NAMESPACE
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println("===========================================")
	log.Println("starting child process")
	log.Println("===========================================")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("while running cmd: %w", err)
	}
	return nil
}

func child() error {
	log.Println("child: new pid, UTS, namespace")
	if err := profile(); err != nil {
		return err
	}

	// create cgroup to restrict resource usage of container
	minMem := int64(1024)         // 1K
	maxMem := int64(100*1024 ^ 2) //100M
	res := cgroupsv2.Resources{
		Memory: &cgroupsv2.Memory{
			// values are in bytes: https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#memory-interface-files
			Min: &minMem,
			Max: &maxMem,
		},
	}
	mgr, err := cgroupsv2.NewManager("/", "/go-container-cgroup", &res)
	if err != nil {
		return fmt.Errorf("creating cgroups v2: %w", err)
	}
	defer mgr.Delete()

	log.Println("cgroups v2 /go-container-cgroup created successfully!")
	if err := profile(); err != nil {
		return err
	}
	// setup for pivot root
	log.Println("mkdir newroot/putold")
	if err := os.MkdirAll("newroot/putold", 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	log.Println("bind mount to ./newroot")
	if err := syscall.Mount("newroot", "newroot", "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind mounting newroot: %w", err)
	}
	if err := profile(); err != nil {
		return err
	}

	log.Println("run pivot_root")
	if err := syscall.PivotRoot("./newroot", "./newroot/putold"); err != nil {
		return fmt.Errorf("pivot root: %w", err)
	}
	if err := profile(); err != nil {
		return err
	}

	log.Println("chdir / and go inside pivot root jail")
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("change dir to /: %w", err)
	}
	if err := profile(); err != nil {
		return err
	}
	// TIPS: by unmounting, parent resource will be hidden from child process
	if err := syscall.Unmount("/putold", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %w", err)
	}
	if err := profile(); err != nil {
		return err
	}

	// mouting /proc inside container
	log.Println("mount /proc")
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mounting new /proc in container: %w", err)
	}
	if err := profile(); err != nil {
		return err
	}
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// this is the start of container process
	log.Println("===========================================")
	log.Printf("running given command on container: %v", os.Args[2:])
	log.Println("===========================================")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("while running cmd: %w", err)
	}
	return nil
}
