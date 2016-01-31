package server

import (
	"io/ioutil"
	"net/http"

	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/helpers"
	rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var RconListener *rcon.RconChatListener

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

	if config.Constants.PortMQ != "" {
		RconListener, err = rcon.NewRconChatListener(ip, config.Constants.PortMQ)
		if err != nil {
			helpers.Logger.Fatal(err)
		}

		helpers.Logger.Info("Listening for server messages on %s:%s (through MQ)", ip, config.Constants.PortMQ)
	} else {
		RconListener, err = rcon.NewRconChatListener(ip, config.Constants.PortRcon)
		if err != nil {
			helpers.Logger.Fatal(err)
		}
		helpers.Logger.Info("Listening for server messages on %s:%s", ip, config.Constants.PortRcon)
	}
}
