default: static

static:
	go build -race -tags "netgo" -ldflags "-linkmode external -extldflags -static" -v -o pauling
clean:
	rm -rf pauling
