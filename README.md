# go-container

Build your own container with golang.

## Feature

- container with new PID, UTS, NAMESPACE
- cgroups
- pivot_root jail

## Run

```bash
git clone git@github.com:go-zen-chu/go-container.git && cd go-container
make download-alpine
GOARCH=amd64 GOOS=linux go build ./main.go

# this binary only supports running on linux
docker run -it --privileged --rm -v $PWD:/go-container -w /go-container alpine:latest /bin/sh

/go-container # ./main run /bin/sh
...
2020/03/22 06:32:08 running given command on container: [/bin/sh]
/ # ls
bin     home    mnt     putold  sbin    tmp                      
dev     lib     opt     root    srv     usr                      
etc     media   proc    run     sys     var
```

## Description

Please refer to my blog post -> (Japanese)

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
