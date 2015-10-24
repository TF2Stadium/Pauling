package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
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
	file, err := os.Open(configPath + "/" + path)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(file)
	line, err := reader.ReadString('\n')

	for err != io.EOF {
		if strings.HasSuffix(line, "exec ") {
			cfgName := line[strings.Index(line, "exec ")+1 : len(line)-1]
			ExecFile(cfgName+".cfg", rcon)
		} else {
			_, rconErr := rcon.Query(line)
			if rconErr != nil {
				return rconErr
			}
		}
		line, err = reader.ReadString('\n')
	}

	file.Close()
	return nil
}
