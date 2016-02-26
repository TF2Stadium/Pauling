package main

import (
	"io/ioutil"
	"net/http"

	"github.com/DSchalla/go-pid"
	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/database"
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
	config.InitConstants()
	helpers.InitLogger()

	// var u *url.URL

	// if config.Constants.AddrMQCtl != "" {
	// 	var err error

	// 	u, err = url.Parse(config.Constants.AddrMQCtl)
	// 	if err != nil {
	// 		helpers.Logger.Fatal(err)
	// 	}
	// }

	if config.Constants.ProfilerEnable {
		address := "localhost:" + config.Constants.PortProfiler
		go func() {
			helpers.Logger.Error(http.ListenAndServe(address, nil).Error())
		}()
		helpers.Logger.Info("Running Profiler on %s", address)
	}

	database.Connect()

	pid := &pid.Instance{}
	if pid.Create() == nil {
		defer pid.Remove()
	}

	server.StartListener()
	server.CreateDB()

	rpc.StartRPC(config.Constants.RabbitMQURL)
}
