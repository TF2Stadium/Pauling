package server

import (
	"io/ioutil"
	"net/http"

	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/helpers"
	rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var Listener *rcon.Listener

func getlocalip() string {
	resp, err := http.Get("http://api.ipify.org")
	if err != nil {
		helpers.Logger.Fatal(err)
	}
	bytes, _ := ioutil.ReadAll(resp.Body)
	return string(bytes)
}

var externalIP = getlocalip()

func StartListener() {
	var err error

	Listener, err = rcon.NewListenerAddr(config.Constants.LogsPort, externalIP+":"+config.Constants.LogsPort, config.Constants.PrintLogMessages)

	if err != nil {
		helpers.Logger.Fatal(err)
	}

	helpers.Logger.Info("Listening for server messages on %s:%s", externalIP, config.Constants.LogsPort)

	connectMQ()
}
