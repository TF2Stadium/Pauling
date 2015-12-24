package main

import (
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"

	"github.com/TF2Stadium/Pauling/internal"
	"github.com/TF2Stadium/Pauling/internal/db"
	"github.com/TF2Stadium/Pauling/internal/helpers"

	"github.com/DSchalla/go-pid"
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
	rpc.StartRPC()
	//PushEvent("getServers")
}
