package server

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

var rMapName = regexp.MustCompile(`^\w+(_+)*\w*$`)
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

func FormatConfigName(lobbyType models.LobbyType) string {
	return fmt.Sprintf("formats/%s.cfg", formatMap[lobbyType])
}

func stripComments(s string) string {
	i := strings.Index(s, "//")
	if i == -1 {
		return s
	} else {
		return s[0:i]
	}
}

//Execute file located at path on rcon
//TODO: Shouldn't this be in TF2RconWrapper?
func ExecFile(path string, rcon *tf2rcon.TF2RconConnection) error {
	configPath, _ := filepath.Abs("./configs/")
	data, err := ioutil.ReadFile(configPath + "/" + path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(stripComments(line))

		err := rcon.QueryNoResp(line)
		if err != nil {
			return err
		}
	}
	return nil

}
