package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/rpc"
	"os"
	"time"

	"github.com/DSchalla/go-pid"
	rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var (
	RconListener     *rcon.RconChatListener
	PrintLogMessages bool
)

func overrideFromEnv(constant *string, name string) {
	val := os.Getenv(name)
	if "" != val {
		*constant = val
	}
}

func overrideBoolFromEnv(constant *bool, name string) {
	val := os.Getenv(name)
	if val != "" {
		*constant = map[string]bool{
			"true":  true,
			"false": false,
		}[val]
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

	var profilerEnable bool
	profilerPort := "6061"
	overrideFromEnv(&profilerPort, "PROFILER_PORT")
	overrideBoolFromEnv(&profilerEnable, "PROFILER_ENABLE")
	overrideBoolFromEnv(&PrintLogMessages, "PRINT_LOG_MESSAGES")

	if profilerEnable {
		address := "localhost:" + profilerPort
		go func() {
			Logger.Error(http.ListenAndServe(address, nil).Error())
		}()
		Logger.Debug("Running Profiler on %s", address)
	}

	pid := &pid.Instance{}
	if pid.Create() == nil {
		defer pid.Remove()
	}

	pauling := new(Pauling)
	rpc.Register(pauling)
	rpc.HandleHTTP()
	port := "8001"
	portRcon := "8002"
	overrideFromEnv(&port, "PAULING_PORT")
	overrideFromEnv(&portRcon, "RCON_PORT")
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%s", port))
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
