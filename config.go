package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/TF2Stadium/Helen/models"
	rcon "github.com/TF2Stadium/TF2RconWrapper"
)

var formatMap = map[models.LobbyType]string{
	models.LobbyTypeSixes:      "6v6",
	models.LobbyTypeHighlander: "9v9",
}

var ConfigsPath = "/src/github.com/TF2Stadium/Pauling/configs/"
var MapsFile = "maps.json"

const (
	LeagueUgc   = "ugc"
	LeagueEtf2l = "etf2l"
)

//Maps the Map, Format and League to a config
var configMap map[string]map[string]map[string]string

var ErrMapNotFound = errors.New("No config for this map was found")
var ErrLobbyTypeNotFound = errors.New("No config for this format was found")
var ErrLeagueNotFound = errors.New("No config for this league was found")

//Initialize the config map
func InitConfigs() {
	overrideFromEnv(&ConfigsPath, "PAULING_CONFIGS_PATH")
	overrideFromEnv(&MapsFile, "PAULING_MAPS")
	file, err := ioutil.ReadFile(os.Getenv("GOPATH") + ConfigsPath + MapsFile)
	if err != nil {
		Logger.Fatal("Error while loading MapsFile: ", err)
	}
	err = json.Unmarshal(file, &configMap)
	if err != nil {
		Logger.Fatal(err)
	}
}

//Return a corresponding config file name for a given map, LobbyType, and League
func ConfigFileName(mapName string, lobbyType models.LobbyType, league string) (string, error) {
	lobbyTypeMap, ok := configMap[mapName]
	if !ok {
		return "", ErrMapNotFound
	}
	leagueMap, ok := lobbyTypeMap[formatMap[lobbyType]]
	if !ok {
		return "", ErrLobbyTypeNotFound
	}
	config, ok := leagueMap[league]
	if !ok {
		return "", ErrLeagueNotFound
	}

	return fmt.Sprintf("%s/%s/%s_%s_%s.cfg", ConfigsPath, string(league), string(league), formatMap[lobbyType], config), nil
}

//Execute file located at path on rcon
//TODO: Shouldn't this be in TF2RconWrapper?
func ExecFile(path string, rcon *rcon.TF2RconConnection) error {
	file, err := os.Open(os.Getenv("GOPATH") + path)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(file)
	line, err := reader.ReadString('\n')
	for err != io.EOF {
		_, rconErr := rcon.Query(line)
		if rconErr != nil {
			return rconErr
		}
		line, err = reader.ReadString('\n')
	}

	return nil
}
