# go-container

Build your own container with golang.

## Description

Please refer to my blog post -> (Japanese)

## Run

## FAQ

### cannot build on my Mac

When you `go run main.go` on MacOS, you'll get error as below.

```bash
# github.com/containerd/cgroups
../../go/pkg/mod/github.com/containerd/cgroups@v0.0.0-20200226104544-44306b6a1d46/memory.go:211:33: undefined: unix.SYS_EVENTFD2
../../go/pkg/mod/github.com/containerd/cgroups@v0.0.0-20200226104544-44306b6a1d46/memory.go:211:55: undefined: unix.EFD_CLOEXEC
../../go/pkg/mod/github.com/containerd/cgroups@v0.0.0-20200226104544-44306b6a1d46/utils.go:67:8: undefined: unix.CGROUP2_SUPER_MAGIC
../../go/pkg/mod/github.com/containerd/cgroups@v0.0.0-20200226104544-44306b6a1d46/utils.go:74:18: undefined: unix.CGROUP2_SUPER_MAGIC
```

This is because cgroups uses Linux kernel function. Build with `GOARCH=amd64 GOOS=linux go build`
