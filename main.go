package main

import (
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"

	"github.com/DSchalla/go-pid"
	rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var (
	RconListener     *rcon.RconChatListener
	PrintLogMessages bool
)

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
		Logger.Info("Running Profiler on %s", address)
	}

	pid := &pid.Instance{}
	if pid.Create() == nil {
		defer pid.Remove()
	}

	portRcon := "8002"
	overrideFromEnv(&portRcon, "RCON_PORT")

	ip := getlocalip()
	var err error
	RconListener, err = rcon.NewRconChatListener(ip, portRcon)
	if err != nil {
		Logger.Fatal(err)
	}

	Logger.Info("Listening for server messages on %s:%s", ip, portRcon)
	startRPC()
	//PushEvent("getServers")
}
