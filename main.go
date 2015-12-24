package main

import (
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"

	"github.com/DSchalla/go-pid"
	"github.com/TF2Stadium/Pauling/db"
	"github.com/TF2Stadium/Pauling/helpers"
	rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var (
	RconListener *rcon.RconChatListener
)

func getlocalip() string {
	resp, err := http.Get("http://api.ipify.org")
	if err != nil {
		helpers.Logger.Fatal(err)
	}
	bytes, _ := ioutil.ReadAll(resp.Body)
	return string(bytes)
}

func main() {
	db.ConnectDB()

	if helpers.ProfilerEnable {
		address := "localhost:" + helpers.PortProfiler
		go func() {
			helpers.Logger.Error(http.ListenAndServe(address, nil).Error())
		}()
		helpers.Logger.Info("Running Profiler on %s", address)
	}

	pid := &pid.Instance{}
	if pid.Create() == nil {
		defer pid.Remove()
	}

	helpers.Logger.Debug("Getting IP Address")
	ip := getlocalip()
	var err error
	RconListener, err = rcon.NewRconChatListener(ip, helpers.PortRcon)
	if err != nil {
		helpers.Logger.Fatal(err)
	}

	helpers.Logger.Info("Listening for server messages on %s:%s", ip, helpers.PortRcon)
	startRPC()
	//PushEvent("getServers")
}
