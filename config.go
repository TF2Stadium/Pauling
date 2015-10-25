package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/TF2Stadium/Helen/models"
	rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var formatMap = map[models.LobbyType]string{
	models.LobbyTypeSixes:      "sixes",
	models.LobbyTypeHighlander: "highlander",
}

func ConfigName(mapName string, lobbyType models.LobbyType, ruleset string) string {
	var file string
	mapType := mapName[:strings.Index(mapName, "_")]

	file = fmt.Sprintf("%s/%s_%s.cfg", ruleset, mapType, formatMap[lobbyType])
	return file
}

//Execute file located at path on rcon
//TODO: Shouldn't this be in TF2RconWrapper?
func ExecFile(path string, rcon *rcon.TF2RconConnection) error {
	configPath, _ := filepath.Abs("./configs/")
	data, err := ioutil.ReadFile(configPath + "/" + path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		_, rconErr := rcon.Query(line)
		if rconErr != nil {
			return rconErr

		}
	}
	return nil
}
