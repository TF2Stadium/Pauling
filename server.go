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

	PlayersRep struct {
		Map map[string]bool
		*sync.RWMutex
	}

	StopVerifier    chan struct{}
	StopLogListener chan struct{}

	ServerListener *TF2RconWrapper.ServerListener

	Rcon *TF2RconWrapper.TF2RconConnection
	Info models.ServerRecord
}

func NewServer() *Server {
	s := &Server{
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

		PlayersRep: struct {
			Map map[string]bool
			*sync.RWMutex
		}{make(map[string]bool), new(sync.RWMutex)},

		StopVerifier:    make(chan struct{}),
		StopLogListener: make(chan struct{}),
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

	for {
		select {
		case <-ticker.C:
			if !s.Verify() {
				ticker.Stop()
				s.Rcon.Close()
				deleteServer(s.LobbyId)
				return
			}
		case <-s.StopVerifier:
			Logger.Debug("Stopping logger for lobby %d", s.LobbyId)
			s.Rcon.Say("[tf2stadium.com] Lobby Ended.")
			ticker.Stop()
			s.ServerListener.Close(s.Rcon)
			s.Rcon.Close()
			deleteServer(s.LobbyId)
			return
		}
	}
}

func (s *Server) LogListener() {
	//Steam IDs used in Source logs are of the form [C:U:A]
	//We convert these into a 64-bit Steam Community ID, which
	//is what Helen uses (and is sent in RPC calls)
	for {
		select {
		case raw := <-s.ServerListener.RawMessages:
			message, err := TF2RconWrapper.ParseMessage(raw)
			if PrintLogMessages {
				Logger.Debug(message.Message)
			}
			//Logger.Debug(message.Message)

			if err != nil {
				continue
			}

			switch message.Parsed.Type {
			case TF2RconWrapper.WorldGameOver:
				PushEvent(EventMatchEnded, s.LobbyId)
				s.StopVerifier <- struct{}{}
				return
			case TF2RconWrapper.PlayerGlobalMessage:
				text := message.Parsed.Data.Text
				//Logger.Debug("GLOBAL %s:", text)

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

		case <-s.StopLogListener:
			s.StopVerifier <- struct{}{}
			return

		}
	}
}

func (s *Server) Setup() error {
	if config.Constants.ServerMockUp {
		return nil
	}

	Logger.Debug("#%d: Connecting to %s", s.LobbyId, s.Info.Host)

	var err error
	s.Rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
	if err != nil {
		return err
	}

	// kick players
	Logger.Debug("#%d: Kicking all players", s.LobbyId)
	kickErr := s.KickAll()

	if kickErr != nil {
		return kickErr
	}

	Logger.Debug("#%d: Setting whitelist", s.LobbyId)
	// whitelist
	_, err = s.Rcon.Query(fmt.Sprintf("tftrue_whitelist_id %d", s.Whitelist))
	if err == TF2RconWrapper.ErrUnknownCommand {
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

	name, err := ConfigName(s.Map, s.Type, s.League)
	if err != nil {
		return err
	}

	filePath, _ := filepath.Abs("./configs/" + name)

	f, err := os.Open(filePath)
	if err != nil {
		//Logger.Debug("%s %s", filePath, err.Error())
		return errors.New("Config doesn't exist.")
	}
	f.Close()

	Logger.Debug("#%d: Creating listener", s.LobbyId)
	s.ServerListener = RconListener.CreateServerListener(s.Rcon)
	go s.LogListener()

	// change map,
	Logger.Debug("#%d: Changing Map", s.LobbyId)
	mapErr := s.Rcon.ChangeMap(s.Map)

	if mapErr != nil {
		return mapErr
	}

	Logger.Debug("#%d: Executing config.", s.LobbyId)
	err = s.ExecConfig()
	if err != nil {
		return err

	}

	Logger.Debug("#%d: Configured", s.LobbyId)
	return nil
}

func (s *Server) ExecConfig() error {
	var err error

	filePath, err := ConfigName(s.Map, s.Type, s.League)
	if err != nil {
		return err
	}

	if s.Type != models.LobbyTypeDebug {
		err = ExecFile("base.cfg", s.Rcon)
		if err != nil {
			return err
		}
	}

	err = ExecFile(filePath, s.Rcon)

	if s.Type != models.LobbyTypeDebug {
		err = ExecFile("after_format.cfg", s.Rcon)
		if err != nil {
			return err
		}
	}

	return err
}

// runs each 10 sec
func (s *Server) Verify() bool {
	//Logger.Debug("#%d: Verifying %s...", s.LobbyId, s.Info.Host)
	password, err := s.Rcon.GetServerPassword()
	if err == nil {
		if password == s.Info.ServerPassword {
			return true
		}
	}

	err = s.Rcon.ChangeServerPassword(s.Info.ServerPassword)
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

func (s *Server) KickAll() error {
	_, err := s.Rcon.Query("kickall")

	return err
}

func (s *Server) IsPlayerAllowed(commId string) bool {
	s.AllowedPlayers.RLock()
	defer s.AllowedPlayers.RUnlock()

	if _, ok := s.AllowedPlayers.Map[commId]; ok {
		return true
	}

	return false
}

var (
	rReport        = regexp.MustCompile(`^!rep\s+(.+)\s+(.+)`)
	stopRepTimeout = make(map[uint](chan struct{}))

	repsNeeded = map[models.LobbyType]int{
		models.LobbyTypeSixes:      7,
		models.LobbyTypeDebug:      1,
		models.LobbyTypeHighlander: 7,
		models.LobbyTypeFours:      5,
		models.LobbyTypeBball:      3,
		models.LobbyTypeUltiduo:    3,
	}
)

func (s *Server) clearReps(slot string) {
	s.PlayersRep.Lock()
	for entry, _ := range s.PlayersRep.Map {
		if strings.HasSuffix(entry, slot) {
			delete(s.PlayersRep.Map, entry)
		}
	}
	s.PlayersRep.Unlock()
}

func (s *Server) report(data TF2RconWrapper.PlayerData) {
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

	if team == "blue" {
		team = "blu"
	}

	slot := team + matches[2]
	s.PlayersRep.RLock()
	if s.PlayersRep.Map[data.SteamId+slot] {
		s.PlayersRep.RUnlock()
		return
	}
	s.PlayersRep.RUnlock()

	s.Slots.RLock()
	steamID, ok := s.Slots.Map[slot]
	s.Slots.RUnlock()
	if !ok {
		Logger.Debug("%s doesn't exist in map %v\n", slot, s.Slots.Map)
		return
	}

	s.Reps.Lock()
	s.Reps.Map[steamID]++
	votes := s.Reps.Map[steamID]
	repped := (s.Reps.Map[steamID] == repsNeeded[s.Type])
	if repped {
		s.Reps.Map[steamID] = 0
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
					s.Reps.Map[steamID] = 0
					s.Reps.Unlock()
					s.Rcon.Say("Not sufficient votes after 1 minute, player not reported.")
					s.clearReps(slot)
					return
				case <-c:
					return
				}
			}
		}()
	}

	var player TF2RconWrapper.Player
	players, _ := s.Rcon.GetPlayers()
	for _, p := range players {
		player = p
	}

	if repped {
		steamID, _ = steamid.SteamIdToCommId(steamID)
		PushEvent(EventSubstitute, s.LobbyId, steamID)

		say := fmt.Sprintf("Reported player %s (%s)", player.Username, player.SteamID)
		s.Rcon.Say(say)
		stopRepTimeout[s.LobbyId] <- struct{}{}
		close(stopRepTimeout[s.LobbyId])
		s.clearReps(slot)

	} else {

		say := fmt.Sprintf("Reporting %s (%s): %d/%d votes",
			player.Username, player.SteamID, votes, repsNeeded[s.Type])
		s.Rcon.Say(say)
		s.PlayersRep.Lock()
		s.PlayersRep.Map[data.SteamId+slot] = true
		s.PlayersRep.Unlock()

	}

	return
}
