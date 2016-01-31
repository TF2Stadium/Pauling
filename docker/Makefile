docker:
	go build -ldflags "-linkmode external -extldflags -static" -v  -o pauling ../
	docker build -t tf2stadium/pauling .

clean:
	rm -rf pauling
