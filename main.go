package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/DSchalla/go-pid"
	"github.com/TF2Stadium/Pauling/config"
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
	u, err := url.Parse(config.Constants.AddrMQCtl)
	if err != nil {
		helpers.Logger.Fatal(err)
	}

	u.Path = "start"
	_, err = http.Post(u.String(), "", nil)
	if err != nil {
		helpers.Logger.Fatal(err)
	}

	if config.Constants.ProfilerEnable {
		address := "localhost:" + config.Constants.PortProfiler
		go func() {
			helpers.Logger.Error(http.ListenAndServe(address, nil).Error())
		}()
		helpers.Logger.Info("Running Profiler on %s", address)
	}

	pid := &pid.Instance{}
	if pid.Create() == nil {
		defer pid.Remove()
	}

	server.StartListener()
	server.CreateDB()

	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%s", config.Constants.PortRPC))
	if err != nil {
		helpers.Logger.Fatal(err)
	}
	go rpc.StartRPC(l)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	for {
		<-sig
		if config.Constants.AddrMQCtl != "" {
			helpers.Logger.Info("Received SIGINT/SIGTERM, queuing messages.")
			l.Close()
			u.Path = "stop"
			http.Post(u.String(), "", nil)
			os.Exit(0)
		}
	}
}
