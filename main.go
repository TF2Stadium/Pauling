package main

import (
	"net"
	"net/http"
	"net/rpc"
	"os"
)

func overrideFromEnv(constant *string, envVar string) {
	v := os.Getenv(envVar)
	if v != "" {
		*constant = envVar
	}
}

func main() {
	InitLogger()
	InitConfigs()
	pauling := new(Pauling)
	rpc.Register(pauling)
	rpc.HandleHTTP()
	port := "1234"
	overrideFromEnv(&port, "PAULING_PORT")
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		Logger.Fatal(err)
	}
	PushEvent("getServers")
	Logger.Debug("Listening on %s", port)
	Logger.Fatal(http.Serve(l, nil))
}
