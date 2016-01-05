package helpers

import (
	"io/ioutil"
	"net/http"

	rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var RconListener *rcon.RconChatListener

func getlocalip() string {
	resp, err := http.Get("http://api.ipify.org")
	if err != nil {
		Logger.Fatal(err)
	}
	bytes, _ := ioutil.ReadAll(resp.Body)
	return string(bytes)
}

func init() {
	initLogger()
	Logger.Debug("Getting IP Address")
	ip := getlocalip()
	var err error
	RconListener, err = rcon.NewRconChatListener(ip, PortRcon)
	if err != nil {
		Logger.Fatal(err)
	}

	Logger.Info("Listening for server messages on %s:%s", ip, PortRcon)
}
