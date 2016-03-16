default: static docker

static:
	CGO_ENABLED=0 go build -tags "netgo" -v -o pauling
clean:
	rm -rf pauling
