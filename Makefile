.PHONY: help
## print all available commands
help:
	@./make/help.sh

.PHONY: download-alpine-docker
## export alpine linux status for running sample container
download-alpine-docker:
	docker pull alpine:latest
	docker run --name alpine-tmp alpine:latest
	docker export alpine-tmp > alpine.tar
	docker rm alpine-tmp
	tar -xvf alpine.tar -C newroot/
	rm alpine.tar

.PHONY: download-alpine-lima
download-alpine-lima:
	lima nerdctl pull alpine:latest
	lima nerdctl run --name alpine-tmp alpine:latest
	lima nerdctl export alpine-tmp > alpine.tar
	lima nerdctl rm alpine-tmp
	tar -xvf alpine.tar -C newroot/
	rm alpine.tar