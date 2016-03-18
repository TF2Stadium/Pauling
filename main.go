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

	if config.Constants.ProfilerAddr != "" {
		helpers.Logger.Info("Running profiler on %s", config.Constants.ProfilerAddr)
		go func() { helpers.Logger.Errorf("%v", http.ListenAndServe(config.Constants.ProfilerAddr, nil)) }()

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
