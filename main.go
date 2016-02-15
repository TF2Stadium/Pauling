package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/DSchalla/go-pid"
	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/database"
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/TF2Stadium/Pauling/rpc"
	"github.com/TF2Stadium/Pauling/server"
	"github.com/TF2Stadium/etcd"
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

	if config.Constants.EtcdAddr != "" {
		err := etcd.ConnectEtcd(config.Constants.EtcdAddr)
		if err != nil {
			helpers.Logger.Fatal(err)
		}

		node, err := etcd.SetAddr(config.Constants.EtcdService, config.Constants.RPCAddr)
		if err != nil {
			helpers.Logger.Fatal(err)
		}

		helpers.Logger.Info("Written key %s=%s", node.Key, node.Value)
	}

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

	l, err := net.Listen("tcp", config.Constants.RPCAddr)
	if err != nil {
		helpers.Logger.Fatal(err)
	}
	go rpc.StartRPC(l)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	for {
		<-sig
		// if config.Constants.AddrMQCtl != "" {
		// 	helpers.Logger.Info("Received SIGINT/SIGTERM, queuing messages.")
		// 	l.Close()
		// 	u.Path = "stop"
		// 	http.Post(u.String(), "", nil)
		// }
		l.Close()
		os.Exit(0)
	}
}
