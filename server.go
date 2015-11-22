package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/TF2Stadium/Helen/config"
	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
	"github.com/TF2Stadium/TF2RconWrapper"
)

var NewServerChan = make(chan *Server)

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

	Slots struct {
		Map map[string]string
		*sync.RWMutex
	}

	Reps struct {
		Map map[string]int
		*sync.RWMutex
	}

	StopVerifier chan struct{}

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

		Slots: struct {
			Map map[string]string
			*sync.RWMutex
		}{make(map[string]string), new(sync.RWMutex)},
		StopVerifier: make(chan struct{}),
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
	var err error
	var count int

	s.Rcon.Close()
	s.Rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
	for err != nil && count != 5 {
		time.Sleep(1 * time.Second)
		Logger.Critical(err.Error())
		count++
		s.Rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
	}
	if count == 5 {
		PushEvent(EventDisconectedFromServer, s.LobbyId)
		return
	}
	s.ServerListener = RconListener.CreateServerListener(s.Rcon)
	go s.LogListener()

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
			s.Rcon.Query("[tf2stadium.com] Lobby Ended.")
			ticker.Stop()
			RconListener.Close(s.Rcon)
			s.Rcon.Close()
			return
		}
	}
}

func (s *Server) LogListener() {
	//Steam IDs used in Source logs are of the form [C:U:A]
	//We convert these into a 64-bit Steam Community ID, which
	//is what Helen uses (and is sent in RPC calls)
	for {
		message := <-s.ServerListener.Messages

		switch message.Parsed.Type {
		case TF2RconWrapper.WorldGameOver:
			PushEvent(EventMatchEnded, s.LobbyId)
			close(s.StopVerifier)
			return
		case TF2RconWrapper.PlayerGlobalMessage:
			text := message.Parsed.Data.Text
			if strings.HasPrefix(text, "!rep") {
				s.report(message.Parsed.Data)
			} else if strings.HasPrefix(text, "!sub") {
				commID, _ := steamid.SteamIdToCommId(message.Parsed.Data.SteamId)
				PushEvent(EventSubstitute, s.LobbyId, commID)
				say := fmt.Sprintf("Reporting player %s (%s)",
					message.Parsed.Data.Username, message.Parsed.Data.SteamId)
				s.Rcon.Say(say)
			}
		case TF2RconWrapper.WorldPlayerConnected:
			commID, _ := steamid.SteamIdToCommId(message.Parsed.Data.SteamId)
			if s.IsPlayerAllowed(commID) {
				PushEvent(EventPlayerConnected, s.LobbyId, commID)
			} else {
				s.Rcon.KickPlayerID(message.Parsed.Data.UserId,
					"[tf2stadium.com] You're not in the lobby...")
			}
		case TF2RconWrapper.WorldPlayerDisconnected:
			commID, _ := steamid.SteamIdToCommId(message.Parsed.Data.SteamId)
			if s.IsPlayerAllowed(commID) {
				PushEvent(EventPlayerConnected, s.LobbyId, commID)
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
	_, err := s.Rcon.Query(fmt.Sprintf("tftrue_whitelist_id %d", s.Whitelist))
	if err == TF2RconWrapper.UnknownCommandError {
		var whitelist string

		switch s.Whitelist {
		case 3250:
			whitelist = "etf2l_whitelist_9v9.txt"
		case 4498:
			whitelist = "etf2l_whitelist_6v6.txt"
		case 3312:
			whitelist = "etf2l_whitelist_ultiduo.txt"
		case 3759:
			whitelist = "etf2l_whitelist_bball.txt"

		case 3951:
			whitelist = "item_whitelist_ugc_HL.txt"
		case 4559:
			whitelist = "item_whitelist_ugc_6v6.txt"
		case 3771:
			whitelist = "item_whitelist_ugc_4v4.txt"

		case 3688:
			whitelist = "esea/item_whitelist.txt"
			// case 4034:

			// case 3872:
		}
		s.Rcon.Query("mp_tournament_whitelist " + whitelist)
	}

	filePath, _ := filepath.Abs("./configs/" + ConfigName(s.Map, s.Type, s.League))

	f, err := os.Open(filePath)
	if err != nil {
		//Logger.Debug("%s %s", filePath, err.Error())
		return errors.New("Config doesn't exist.")
	}
	f.Close()

	// change map
	mapErr := s.Rcon.ChangeMap(s.Map)

	if mapErr != nil {
		return mapErr
	}

	// run config
	s.ExecConfig()

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
	//Logger.Debug("#%d: Verifying %s...", s.LobbyId, s.Info.Host)
	var err = s.Rcon.ChangeServerPassword(s.Info.ServerPassword)

	retries := 0
	for err != nil { //TODO: Stop connection after x retries
		if retries == 6 {
			//Logger.Warning("#%d: Couldn't query %s after 5 retries", s.LobbyId, s.Info.Host)
			PushEvent(EventDisconectedFromServer, s.LobbyId)
			return false
		}
		retries++
		time.Sleep(time.Second)
		err = s.Rcon.ChangeServerPassword(s.Info.ServerPassword)
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

var rReport = regexp.MustCompile(`^!rep\s+(.+)\s+(.+)`)
var stopRepTimeout = make(map[uint](chan struct{}))

func (s *Server) report(data TF2RconWrapper.PlayerData) {
	s.Players.RLock()
	defer s.Players.RUnlock()

	var team string
	matches := rReport.FindStringSubmatch(data.Text)
	if len(matches) != 3 {
		return
	}

	switch matches[1] {
	case "our":
		team = strings.ToLower(data.Team)
	case "their":
		our := strings.ToLower(data.Team)
		if our == "red" {
			team = "blu"
		} else {
			team = "red"
		}
	}

	slot := team + matches[2]

	s.Slots.RLock()
	steamid, ok := s.Slots.Map[slot]
	s.Slots.RUnlock()
	if !ok {
		return
	}

	var repped bool
	s.Reps.Lock()
	s.Reps.Map[steamid]++
	votes := s.Reps.Map[steamid]
	repped = (s.Reps.Map[steamid] == 7)
	if repped {
		s.Reps.Map[steamid] = 0
	}
	s.Reps.Unlock()

	if votes == 1 {
		c := make(chan struct{})
		stopRepTimeout[s.LobbyId] = c
		tick := time.After(time.Minute * 1)

		go func() {
			for {
				select {
				case <-tick:
					s.Reps.Lock()
					s.Reps.Map[steamid] = 0
					s.Reps.Unlock()
					s.Rcon.Say("Not sufficient votes after 1 minute, player not reported.")
					return
				case <-c:
					return
				}
			}
		}()
	}

	var player TF2RconWrapper.Player

	if repped {
		PushEvent(EventSubstitute, steamid, s.LobbyId)

		for _, p := range s.Players.Slice {
			player = p
			say := fmt.Sprintf("Reported player %s (%s)", p.Username, p.SteamID)
			s.Rcon.Say(say)

			stopRepTimeout[s.LobbyId] <- struct{}{}
			return
		}
	} else {
		say := fmt.Sprintf("Reporting %s (%s): %d/7 votes",
			player.Username, player.SteamID, votes)
		s.Rcon.Say(say)
	}
}
