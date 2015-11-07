package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"time"

	rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var RconListener *rcon.RconChatListener

func overrideFromEnv(constant *string, envVar string) {
	v := os.Getenv(envVar)
	if v != "" {
		*constant = envVar
	}
}

func getlocalip() string {
	resp, err := http.Get("http://api.ipify.org")
	if err != nil {
		Logger.Fatal(err)
	}
	bytes, _ := ioutil.ReadAll(resp.Body)
	return string(bytes)
}

func main() {
	InitLogger()
	pauling := new(Pauling)
	rpc.Register(pauling)
	rpc.HandleHTTP()
	port := "8001"
	portRcon := "8002"
	overrideFromEnv(&port, "PAULING_PORT")
	overrideFromEnv(&portRcon, "RCON_PORT")
	l, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		Logger.Fatal(err)
	}

	ip := getlocalip()
	RconListener, err = rcon.NewRconChatListener(ip, portRcon)
	if err != nil {
		Logger.Fatal(err)
	}

	Logger.Debug("Listening for server messages on %s:%s", ip, portRcon)
	//PushEvent("getServers")
	go func() {
		for {
			s := <-NewServerChan
			go s.StartVerifier(time.NewTicker(time.Second * 10))

		}
	}()
	Logger.Debug("Listening on %s", port)
	Logger.Fatal(http.Serve(l, nil))
}
