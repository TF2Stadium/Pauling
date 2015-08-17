package main

import (
	"net"
	"net/http"
	"net/rpc"
	"os"
)

func main() {
	pauling := new(Pauling)
	rpc.Register(pauling)
	rpc.HandleHTTP()
	port := os.Getenv("PAULING_PORT")
	if port == "" {
		port = "1234"
	}

	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		Logger.Fatal(err)
	}

	Logger.Debug("Listening on %s", port)
	Logger.Fatal(http.Serve(l, nil))
}
