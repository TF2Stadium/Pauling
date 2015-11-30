package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/TF2Stadium/Helen/models"
	tf2rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var formatMap = map[models.LobbyType]string{
	models.LobbyTypeSixes:      "sixes",
	models.LobbyTypeHighlander: "highlander",
	models.LobbyTypeBball:      "bball",
	models.LobbyTypeUltiduo:    "ultiduo",
	models.LobbyTypeFours:      "fours",
	models.LobbyTypeDebug:      "debug",
}

var rMapName = regexp.MustCompile(`(.+)_(.+)`)
var ErrInvalidMap = errors.New("Invalid Map Name.")

func ConfigName(mapName string, lobbyType models.LobbyType, ruleset string) (string, error) {
	if !rMapName.MatchString(mapName) {
		return "", ErrInvalidMap
	}

	mapType := mapName[:strings.Index(mapName, "_")]
	formatString := formatMap[lobbyType]

	if strings.HasPrefix(mapName, "ultiduo") {
		mapType = "koth"
	}

	file := fmt.Sprintf("%s/%s_%s.cfg", ruleset, mapType, formatString)
	return file, nil
}

//Execute file located at path on rcon
//TODO: Shouldn't this be in TF2RconWrapper?
func ExecFile(path string, rcon *tf2rcon.TF2RconConnection) error {
	configPath, _ := filepath.Abs("./configs/")
	data, _ := ioutil.ReadFile(configPath + "/" + path)

	lines := strings.Split(string(data), "\n")

	var config string
	for _, line := range lines {
		if len(config+line) > 1024-10 {
			str, err := rcon.Query(config)
			if err != nil {
				return errors.New(str)
			}
			config = ""
		}
		config += line + "; "
	}

	Logger.Debug(config)
	str, err := rcon.Query(config)
	if err != nil {
		return errors.New(str)
	}
	return nil

}
