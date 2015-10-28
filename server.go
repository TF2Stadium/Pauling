package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/TF2Stadium/Helen/config"
	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
	"github.com/TF2Stadium/TF2RconWrapper"
)

var NewServerChan = make(chan *Server)

var ServerMap = struct {
	Map map[uint]*Server
	*sync.RWMutex
}{make(map[uint]*Server), new(sync.RWMutex)}

type Server struct {
	Map       string // lobby map
	League    string
	Type      models.LobbyType // 9v9 6v6 4v4...
	Whitelist int

	LobbyId uint

	PrevConnected map[string]bool

	Players struct {
		Slice []TF2RconWrapper.Player
		*sync.RWMutex
	}

	AllowedPlayers struct {
		Map map[string]bool
		*sync.RWMutex
	}

	Reps struct {
		Map map[string]int
		*sync.RWMutex
	}

	Substitutes struct {
		Map map[string]string
		*sync.RWMutex
	}

	StopVerifier chan bool

	ServerListener *TF2RconWrapper.ServerListener

	Rcon *TF2RconWrapper.TF2RconConnection
	Info models.ServerRecord
}

func NewServer() *Server {
	s := &Server{
		Players: struct {
			Slice []TF2RconWrapper.Player
			*sync.RWMutex
		}{make([]TF2RconWrapper.Player, 4), new(sync.RWMutex)},

		AllowedPlayers: struct {
			Map map[string]bool
			*sync.RWMutex
		}{make(map[string]bool), new(sync.RWMutex)},

		Reps: struct {
			Map map[string]int
			*sync.RWMutex
		}{make(map[string]int), new(sync.RWMutex)},

		Substitutes: struct {
			Map map[string]string
			*sync.RWMutex
		}{make(map[string]string), new(sync.RWMutex)},

		StopVerifier: make(chan bool),
	}

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

func (s *Server) StartVerifier(ticker *time.Ticker) {
	s.Rcon.Close()
	s.Rcon, _ = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
	// run config
	s.ExecConfig()
	s.ServerListener = RconListener.CreateServerListener(s.Rcon)
	go s.CommandListener()

	for {
		select {
		case <-ticker.C:
			if !s.Verify() {
				ticker.Stop()
				s.Rcon.Close()
				return
			}
		case <-s.StopVerifier:
			Logger.Debug("Stopping logger for lobby %d", s.LobbyId)
			ticker.Stop()
			RconListener.Close(s.Rcon)
			s.Rcon.Close()
			return
		}
	}
}

func (s *Server) CommandListener() {
	for {
		message := <-s.ServerListener.Messages
		if message.Parsed.Type == TF2RconWrapper.WorldGameOver {
			PushEvent(EventMatchEnded, s.LobbyId)
			s.StopVerifier <- true
			return
		}

		if message.Parsed.Type == TF2RconWrapper.PlayerGlobalMessage {
			text := message.Parsed.Data.Text
			if strings.HasPrefix(text, "!rep") {
				s.report(text[5:])
			} else if strings.HasPrefix(text, "!sub") {
				commid, _ := steamid.SteamIdToCommId(message.Parsed.Data.SteamId)
				s.Substitutes.Lock()
				s.Substitutes.Map[commid] = ""
				s.Substitutes.Unlock()
				PushEvent(EventSubstitute, commid)
			}
		}

	}
}

func (s *Server) Setup() error {
	if config.Constants.ServerMockUp {
		return nil
	}

	Logger.Debug("#%d: Connecting to %s", s.LobbyId, s.Info.Host)

	s.Rcon, _ = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)

	// kick players
	Logger.Debug("#%d: Kicking all players", s.LobbyId)
	kickErr := s.KickAll()

	if kickErr != nil {
		return kickErr
	}

	Logger.Debug("#%d: Setting whitelist, changing map", s.LobbyId)
	// whitelist
	s.Rcon.Query(fmt.Sprintf("tftrue_whitelist_id %d", s.Whitelist))

	// change map
	mapErr := s.Rcon.ChangeMap(s.Map)

	if mapErr != nil {
		return mapErr
	}

	return nil
}

func (s *Server) ExecConfig() error {
	filePath := ConfigName(s.Map, s.Type, s.League)

	var err error
	if s.Type != models.LobbyTypeDebug {
		err = ExecFile("base.cfg", s.Rcon)
		if err != nil {
			return err
		}
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
	Logger.Debug("#%d: Verifying %s...", s.Info.Host)
	s.Rcon.ChangeServerPassword(s.Info.ServerPassword)

	// check if all players in server are in lobby
	var err error
	s.Players.Lock()
	s.Players.Slice, err = s.Rcon.GetPlayers()
	s.Players.Unlock()

	retries := 0
	for err != nil { //TODO: Stop connection after x retries
		if retries == 6 {
			Logger.Warning("#%d: Couldn't query %s after 5 retries", s.Info.Host)
			PushEvent(EventDisconectedFromServer, s.LobbyId)
			return false
		}
		retries++
		time.Sleep(time.Second)
		Logger.Warning("Failed to get players in server %s: %s", s.LobbyId, err.Error())
		s.Players.Lock()
		s.Players.Slice, err = s.Rcon.GetPlayers()
		s.Players.Unlock()
	}

	s.Players.RLock()
	defer s.Players.RUnlock()
	for _, player := range s.Players.Slice {
		if player.SteamID != "BOT" {
			commId, idErr := steamid.SteamIdToCommId(player.SteamID)

			if idErr != nil {
				Logger.Debug("[Server.Verify]: ERROR -> %s", idErr)
			}

			isPlayerAllowed := s.IsPlayerAllowed(commId)

			if isPlayerAllowed == false {
				Logger.Debug("#%d: Kicking player: %s %s %s", player.Username, commId, player.SteamID)

				kickErr := s.Rcon.KickPlayer(player, "[tf2stadium.com]: You're not in this lobby...")

				if kickErr != nil {
					Logger.Critical("#%d: Couldn't kick: ", kickErr)
				}
			}

			s.Substitutes.RLock()
			sub, ok := s.Substitutes.Map[commId]
			s.Substitutes.RUnlock()
			if ok && sub != "" {
				if inserver, _ := s.IsPlayerInServer(commId); inserver {
					s.Rcon.KickPlayer(player, "[tf2stadium.com]: You have been substituted.")
				}
			}
			if ingame, exists := s.PrevConnected[player.SteamID]; exists && !ingame {
				s.PrevConnected[player.SteamID] = true
				PushEvent(EventPlayerConnected, player.SteamID)
			}
		}
	}
	for steamid, _ := range s.PrevConnected {
		if ingame, _ := s.IsPlayerInServer(steamid); !ingame {
			PushEvent(EventPlayerDiscconected, steamid)
			delete(s.PrevConnected, steamid)
		}
	}

	return true
}

// check if the given commId is in the server
func (s *Server) IsPlayerInServer(playerCommId string) (bool, error) {
	s.Players.RLock()
	defer s.Players.RUnlock()

	for _, player := range s.Players.Slice {
		commId, idErr := steamid.SteamIdToCommId(player.SteamID)

		if idErr != nil {
			return false, idErr
		}

		if playerCommId == commId {
			return true, nil
		}
	}

	return false, nil
}

func (s *Server) KickAll() error {
	var err error

	s.Players.Lock()
	s.Players.Slice, err = s.Rcon.GetPlayers()

	for err != nil {
		time.Sleep(time.Second)
		Logger.Critical("%d: Failed to get players in  %s: %s", s.LobbyId, err.Error())
		s.Players.Slice, err = s.Rcon.GetPlayers()
	}
	s.Players.Unlock()

	s.Players.RLock()
	defer s.Players.RUnlock()
	for _, player := range s.Players.Slice {
		kickErr := s.Rcon.KickPlayer(player, "[tf2stadium.com]: Setting up lobby...")

		if kickErr != nil {
			return kickErr
		}
	}

	return nil
}

func (s *Server) IsPlayerAllowed(commId string) bool {
	s.AllowedPlayers.RLock()
	defer s.AllowedPlayers.RUnlock()

	if _, ok := s.AllowedPlayers.Map[commId]; ok {
		return true
	}

	return false
}

func (s *Server) report(name string) {
	s.Players.RLock()
	defer s.Players.RUnlock()

	for _, player := range s.Players.Slice {
		if strings.HasPrefix(player.Username, name) {
			commId, _ := steamid.SteamIdToCommId(player.SteamID)

			s.Reps.Lock()
			s.Reps.Map[player.SteamID]++

			if s.Reps.Map[player.SteamID] == 7 {
				s.AllowedPlayers.Lock()
				s.AllowedPlayers.Map[commId] = false
				s.AllowedPlayers.Unlock()

				err := s.Rcon.KickPlayer(player, "[tf2stadium.com]: You have been reported.")
				if err != nil {
					Logger.Critical("#%d: Couldn't kick player: %s", s.LobbyId, err)
				}

				PushEvent(EventPlayerReported, commId, s.LobbyId)
			}
			s.Reps.Unlock()
			return
		}
	}
}
