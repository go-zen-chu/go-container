package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/containerd/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func main() {
	if runtime.GOOS == "linux" {
		log.Println("recognizing as linux")
	} else {
		panic("only linux is supported")
	}
	if len(os.Args) == 1 {
		panic("usage: ./go-container run /bin/sh")
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
}

func profile() error {
	log.Println("[PROFILE] current process status:")
	fmt.Printf("pid: %d, parent pid: %d, euid: %d, uid: %d, egid: %d, gid: %d\n",
		os.Getpid(), os.Getppid(), os.Geteuid(), os.Getuid(), os.Getegid(), os.Getgid())
	if dir, err := os.Getwd(); err != nil {
		return err
	} else {
		fmt.Printf("current dir: %s\n", dir)
		if files, err := ioutil.ReadDir("."); err != nil {
			return err
		} else {
			for _, file := range files {
				fmt.Printf(" |- %s\n", file.Name())
			}
		}
	}
	if _, err := os.Stat("/proc/self"); err == nil {
		if content, err := ioutil.ReadFile("/proc/self/mounts"); err != nil {
			return err
		} else {
			fmt.Printf("mount info:\n%s\n", content)
		}
		if content, err := ioutil.ReadFile("/proc/self/cgroup"); err != nil {
			return err
		} else {
			fmt.Printf("cgroups:\n%s\n", content)
		}
	} else {
		fmt.Println("/proc/self not exists")
	}
	return nil
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	if _, err = io.Copy(d, s); err != nil {
		return err
	}
	if err := d.Chmod(0777); err != nil {
		return err
	}
	return nil
}

// parent function update current process as container
func parent() error {
	log.Println("parent: original process")
	if err := profile(); err != nil {
		return err
	}

	// /proc/self/exe is a special file containing an in-memory image of the current executable in Linux
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	// parameters for making child process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("while running cmd: %s", err)
	}
	return nil
}

func child() error {
	log.Println("child: new pid, UTS, namespace")
	if err := profile(); err != nil {
		return err
	}
	// create cgroup to restrict resource usage of container
	shares := uint64(50)
	control, err := cgroups.New(cgroups.V1, cgroups.StaticPath("/go-container"), &specs.LinuxResources{
		CPU: &specs.LinuxCPU{
			Shares: &shares,
		},
	})
	if err != nil {
		return fmt.Errorf("creating cgroups: %s", err)
	}
	defer control.Delete()
	// restrict self
	if err := control.Add(cgroups.Process{Pid: os.Getpid()}); err != nil {
		return fmt.Errorf("adding cgroups: %s", err)
	}
	log.Println("cgroups go-container has set")
	if err := profile(); err != nil {
		return err
	}
	log.Println("mount newroot")
	if err := os.MkdirAll("/newroot", 0755); err != nil {
		return fmt.Errorf("creating folder: %s", err)
	}
	if err := syscall.Mount("newroot", "/newroot", "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind mounting /newroot: %s", err)
	}
	if err := profile(); err != nil {
		return err
	}
	// log.Println("create putold")
	// if err := os.MkdirAll("newroot/putold", 0700); err != nil {
	// 	return fmt.Errorf("creating folder: %s", err)
	// }
	log.Println("chdir /newroot/putold")
	if err := os.Chdir("/newroot/putold"); err != nil {
		return fmt.Errorf("change dir to /newroot/putold: %s", err)
	}
	if err := profile(); err != nil {
		return err
	}
	log.Println("run pivot_root")
	if err := syscall.PivotRoot("/newroot", "/newroot/putold"); err != nil {
		return fmt.Errorf("pivot root: %s", err)
	}
	if err := profile(); err != nil {
		return err
	}
	log.Println("chroot /putold")
	if err := syscall.Chroot("/putold"); err != nil {
		return fmt.Errorf("chroot: %s", err)
	}
	if err := profile(); err != nil {
		return err
	}

	// This chdir go back to original / (not /newroot/putold)
	// log.Println("chdir /")
	// if err := os.Chdir("/"); err != nil {
	// 	return fmt.Errorf("change dir to /: %s", err)
	// }
	// if err := profile(); err != nil {
	// 	return err
	// }

	//log.Println("chdir /putold")
	// if err := syscall.Chroot("/putold"); err != nil {
	// 	return fmt.Errorf("chroot: %s", err)
	// }
	// log.Println("after chroot")
	// if err := profile(); err != nil {
	// 	return err
	// }
	// if err := os.Chdir("/putold"); err != nil {
	// 	return fmt.Errorf("change dir to /: %s", err)
	// }
	// if err := syscall.Unmount("/", 0); err != nil {
	// 	return fmt.Errorf("unmount %s", err)
	// }
	// log.Println("after umount")
	// if err := profile(); err != nil {
	// 	return err
	// }
	// if err := syscall.Mount("/", "/", "", syscall.MS_BIND, ""); err != nil {
	// 	return fmt.Errorf("mounting new root in container: %s", err)
	// }
	log.Println("mount /proc")
	// mount to /putold/proc
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mounting new /proc in container: %s", err)
	}
	if err := profile(); err != nil {
		return err
	}
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("running given command on container")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("while running cmd: %s", err)
	}
	return nil
}
