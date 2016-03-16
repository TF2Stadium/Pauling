default: static docker

static:
	go build -tags "netgo" -ldflags "-linkmode external -extldflags -static" -v -o pauling
clean:
	rm -rf pauling
