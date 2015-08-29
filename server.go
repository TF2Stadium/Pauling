package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/TF2Stadium/Helen/config"
	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
	"github.com/TF2Stadium/TF2RconWrapper"
)

var LobbyServerMap = make(map[uint]*Server)
var LobbyMutexMap = make(map[uint]*sync.Mutex)

type Server struct {
	Map    string // lobby map
	League string
	Type   models.LobbyType // 9v9 6v6 4v4...

	LobbyId uint

	Players        []TF2RconWrapper.Player // current number of players in the server
	AllowedPlayers map[string]bool

	Ticker verifyTicker // timer that will verify()

	//ChatListener  *TF2RconWrapper.RconChatListener

	Rcon *TF2RconWrapper.TF2RconConnection
	Info models.ServerRecord
}

// timer used in verify()
type verifyTicker struct {
	Ticker *time.Ticker
	Quit   chan bool
}

func (t *verifyTicker) Close() {
	t.Quit <- true
}

func NewServer() *Server {
	s := &Server{}
	s.AllowedPlayers = make(map[string]bool)

	return s
}

// after create the server var, you should run this
//
// things that needs to be specified before run this:
// -> Map
// -> Type
// -> League
// -> Info
//

func (s *Server) StartVerifier() error {
	// If the ticker is initialized, the verifier is running
	if s.Ticker.Quit != nil {
		return nil
	}
	s.Ticker.Ticker = time.NewTicker(10 * time.Second)
	s.Ticker.Quit = make(chan bool)
	go func() {
		for {
			select {
			case <-s.Ticker.Ticker.C:
				if !s.Verify() {
					s.Ticker.Ticker.Stop()
					s.Rcon.Close()
					return
				}
			case <-s.Ticker.Quit:
				s.Ticker.Ticker.Stop()
				return
			}
		}
	}()

	return nil
}

func (s *Server) Setup() error {
	if config.Constants.ServerMockUp {
		return nil
	}

	Logger.Debug("[Server.Setup]: Setting up server -> [" + s.Info.Host + "] from lobby [" + fmt.Sprint(s.LobbyId) + "]")

	// connect to rcon if not connected before
	if s.Rcon == nil {
		var err error
		s.Rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)

		if err != nil {
			return err
		}
	}

	// changing server password
	passErr := s.Rcon.ChangeServerPassword(s.Info.ServerPassword)

	if passErr != nil {
		return passErr
	}

	// kick players
	Logger.Debug("[Server.Setup]: Connected to server, getting players...")
	kickErr := s.KickAll()

	if kickErr != nil {
		return kickErr
	} else {
		Logger.Debug("[Server.Setup]: Players kicked, running config!")
	}

	// run config
	err := s.ExecConfig()
	if err != nil {
		return err
	}

	// change map
	mapErr := s.Rcon.ChangeMap(s.Map)

	if mapErr != nil {
		return mapErr
	}

	return nil
}

func (s *Server) ExecConfig() error {
	filePath, err := ConfigFileName(s.Map, s.Type, s.League)
	if err != nil {
		return err
	}

	err = ExecFile(filePath, s.Rcon)
	if err != nil {
		return err
	}
	return nil
}

// runs each 10 sec
func (s *Server) Verify() bool {
	if config.Constants.ServerMockUp || s.Rcon == nil {
		return true
	}
	Logger.Debug("[Server.Verify]: Verifing server -> [" + s.Info.Host + "] from lobby [" + fmt.Sprint(s.LobbyId) + "]")

	// check if all players in server are in lobby
	var err error
	s.Players, err = s.Rcon.GetPlayers()

	retries := 0
	for err != nil { //TODO: Stop connection after x retries
		if retries == 6 {
			Logger.Warning("[Server.Verify] Couldn't query server [" + s.Info.Host +
				"] after 5 retries")
			PushEvent(EventDisconectedFromServer, s.LobbyId)
			return false
		}
		retries += 1
		time.Sleep(time.Second)
		Logger.Warning("Failed to get players in server %s: %s", s.LobbyId, err.Error())
		s.Players, err = s.Rcon.GetPlayers()
	}

	for i := range s.Players {
		if s.Players[i].SteamID != "BOT" {
			commId, idErr := steamid.SteamIdToCommId(s.Players[i].SteamID)

			if idErr != nil {
				Logger.Debug("[Server.Verify]: ERROR -> %s", idErr)
			}

			isPlayerAllowed := s.IsPlayerAllowed(commId)

			if isPlayerAllowed == false {
				Logger.Debug("[Server.Verify]: Kicking player not allowed -> Username [" +
					s.Players[i].Username + "] CommID [" + commId + "] SteamID [" + s.Players[i].SteamID + "] ")

				kickErr := s.Rcon.KickPlayer(s.Players[i], "[tf2stadium.com]: You're not in this lobby...")

				if kickErr != nil {
					Logger.Debug("[Server.Verify]: ERROR -> %s", kickErr)
				}
			}
		}
	}
	return true
}

// check if the given commId is in the server
func (s *Server) IsPlayerInServer(playerCommId string) (bool, error) {
	for i := range s.Players {
		commId, idErr := steamid.SteamIdToCommId(s.Players[i].SteamID)

		if idErr != nil {
			return false, idErr
		}

		if playerCommId == commId {
			return true, nil
		}
	}

	return false, nil
}

// TODO: get end event from logs
// `World triggered "Game_Over"`
func (s *Server) End() {
	if config.Constants.ServerMockUp {
		return
	}

	Logger.Debug("[Server.End]: Ending server -> [" + s.Info.Host + "] from lobby [" + fmt.Sprint(s.LobbyId) + "]")
	// TODO: upload logs

	PushEvent(EventMatchEnded, s.LobbyId)
	s.Rcon.Close()
	s.Ticker.Close()
}

func (s *Server) KickAll() error {
	Logger.Debug("[Server.KickAll]: Kicking players...")
	var err error
	s.Players, err = s.Rcon.GetPlayers()

	for err != nil {
		time.Sleep(time.Second)
		Logger.Warning("Failed to get players in server %s: %s", s.LobbyId, err.Error())
		s.Players, err = s.Rcon.GetPlayers()
	}

	for i := range s.Players {
		kickErr := s.Rcon.KickPlayer(s.Players[i], "[tf2stadium.com]: Setting up lobby...")

		if kickErr != nil {
			return kickErr
		}
	}

	return nil
}

func (s *Server) IsPlayerAllowed(commId string) bool {
	if _, ok := s.AllowedPlayers[commId]; ok {
		return true
	}

	return false
}
