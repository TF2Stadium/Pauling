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
	models.LobbyTypeBball:      "bball",
	models.LobbyTypeFours:      "fours",
	models.LobbyTypeDebug:      "debug",
}

func ConfigName(mapName string, lobbyType models.LobbyType, ruleset string) string {
	var file string
	mapType := mapName[:strings.Index(mapName, "_")]
	formatString := formatMap[lobbyType]

	if strings.HasPrefix(mapName, "ultiduo") || strings.HasPrefix(mapName, "koth_ultiduo") {
		mapType = "koth"
		formatString = "ultiduo"
	}

	file = fmt.Sprintf("%s/%s_%s.cfg", ruleset, mapType, formatString)
	return file
}

//Execute file located at path on rcon
//TODO: Shouldn't this be in TF2RconWrapper?
func ExecFile(path string, rcon *rcon.TF2RconConnection) error {
	configPath, _ := filepath.Abs("./configs/")
	data, _ := ioutil.ReadFile(configPath + "/" + path)

	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		rcon.Query(line)
	}
	return nil
}
