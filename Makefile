.PHONY: help
## print all available commands
help:
	@./make/help.sh

.PHONY: download-alpine-docker
## export alpine linux status for running sample container
download-alpine-docker:
	rm -rf newroot/*
	docker pull alpine:latest
	docker run --name alpine-tmp alpine:latest
	docker export alpine-tmp > alpine.tar
	docker rm alpine-tmp
	tar -xvf alpine.tar -C newroot/
	rm alpine.tar

.PHONY: run-go-container
run-go-container:
	GOARCH=amd64 GOOS=linux go build main.go
	docker run -it --privileged --rm -v ${PWD}:/go-container -w /go-container alpine:latest /bin/sh -c "./main run /bin/sh"