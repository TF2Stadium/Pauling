default: static docker

static:
	go build -ldflags "-linkmode external -extldflags -static" -v  -o pauling

docker:
	docker build -t tf2stadium/pauling .

clean:
	rm -rf pauling
