package server

import (
	"io/ioutil"
	"net/http"

	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/helpers"
	rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var listener *rcon.Listener

func getlocalip() string {
	resp, err := http.Get("http://api.ipify.org")
	if err != nil {
		helpers.Logger.Fatal(err)
	}
	bytes, _ := ioutil.ReadAll(resp.Body)
	return string(bytes)
}

func StartListener() {
	var err error
	ip := getlocalip()

	listener, err = rcon.NewListenerAddr(config.Constants.LogsPort, ip+":"+config.Constants.LogsPort)

	if err != nil {
		helpers.Logger.Fatal(err)
	}

	helpers.Logger.Info("Listening for server messages on %s:%s", ip, config.Constants.LogsPort)

	connectMQ()
}
