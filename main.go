package main

import (
	"io/ioutil"
	"net/http"

	"github.com/DSchalla/go-pid"
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/TF2Stadium/Pauling/rpc"
	"github.com/TF2Stadium/Pauling/server"
	_ "github.com/rakyll/gom/http"
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
	server.CreateDB()
	rpc.StartRPC()
	//PushEvent("getServers")
}
