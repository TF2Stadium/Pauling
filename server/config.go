package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/TF2Stadium/Helen/models/lobby/format"
	tf2rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var formatMap = map[format.Format]string{
	format.Sixes:      "sixes",
	format.Highlander: "highlander",
	format.Bball:      "bball",
	format.Ultiduo:    "ultiduo",
	format.Fours:      "fours",
	format.Debug:      "debug",
}

var rMapName = regexp.MustCompile(`^\w+(_+)*\w*$`)
var ErrInvalidMap = errors.New("Invalid Map Name.")

func ConfigName(mapName string, lobbyType format.Format, ruleset string) (string, error) {
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

func FormatConfigName(lobbyType format.Format) string {
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
		_, err := rcon.Query(line)
		if err != nil {
			rcon.Reconnect(time.Second * 10)
			if _, err = rcon.Query(line); err != nil {
				continue
			}
		}

	}
	return nil

}
