package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/TF2Stadium/Helen/config"
	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/Pauling/helen"
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
	"github.com/TF2Stadium/TF2RconWrapper"
)

type Server struct {
	Map       string // lobby map
	League    string
	Type      models.LobbyType // 9v9 6v6 4v4...
	Whitelist string

	LobbyId uint

	StopRepTimer    map[string]chan struct{}
	StopVerifier    chan struct{}
	StopLogListener chan struct{}

	ServerListener *TF2RconWrapper.ServerListener

	Rcon *TF2RconWrapper.TF2RconConnection
	Info models.ServerRecord
}

func NewServer() *Server {
	s := &Server{
		StopRepTimer:    make(map[string]chan struct{}),
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

	_, err = s.Rcon.Query("status")
	if err != nil {
		s.Rcon.Close()
		s.Rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
		for err != nil && count != 5 {
			time.Sleep(1 * time.Second)
			helpers.Logger.Critical(err.Error())
			count++
			s.Rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
		}
		if count == 5 {
			DisconnectedFromServer(s.LobbyId)
			s.ServerListener.Close(s.Rcon)
			s.StopLogListener <- struct{}{}
			return
		}
	}
	for {
		select {
		case <-ticker.C:
			if !s.Verify() {
				ticker.Stop()
				s.Rcon.Close()
				DeleteServer(s.LobbyId)
				return
			}
		case <-s.StopVerifier:
			helpers.Logger.Debug("Stopping logger for lobby %d", s.LobbyId)
			s.Rcon.Say("[tf2stadium.com] Lobby Ended.")
			ticker.Stop()
			s.Rcon.Close()
			DeleteServer(s.LobbyId)
			return
		}
	}
}

func (s *Server) logListener() {
	//Steam IDs used in Source logs are of the form [C:U:A]
	//We convert these into a 64-bit Steam Community ID, which
	//is what Helen uses (and is sent in RPC calls)
	for {
		select {
		case raw := <-s.ServerListener.RawMessages:
			message, err := TF2RconWrapper.ParseMessage(raw)
			if helpers.PrintLogMessages {
				helpers.Logger.Debug(message.Message)
			}
			//helpers.Logger.Debug(message.Message)

			if err != nil {
				continue
			}

			switch message.Parsed.Type {
			case TF2RconWrapper.WorldGameOver:
				s.ServerListener.Close(s.Rcon)
				MatchEnded(s.LobbyId)
				s.StopVerifier <- struct{}{}
				return

			case TF2RconWrapper.PlayerGlobalMessage:
				playerData := message.Parsed.Data.(TF2RconWrapper.PlayerData)
				text := playerData.Text
				//Logger.Debug("GLOBAL %s:", text)

				if strings.HasPrefix(text, "!rep") {
					s.report(playerData)
				} else if strings.HasPrefix(text, "!sub") {
					commID, _ := steamid.SteamIdToCommId(playerData.SteamId)
					playerID := helen.GetPlayerID(commID)

					Substitute(s.LobbyId, playerID)

					say := fmt.Sprintf("Reporting player %s (%s)",
						playerData.Username, playerData.SteamId)
					s.Rcon.Say(say)
				}

			case TF2RconWrapper.WorldPlayerConnected:
				playerData := message.Parsed.Data.(TF2RconWrapper.PlayerData)
				commID, _ := steamid.SteamIdToCommId(playerData.SteamId)

				if s.IsPlayerAllowed(commID) {
					playerID := helen.GetPlayerID(commID)
					PlayerConnected(s.LobbyId, playerID)
				} else {
					s.Rcon.KickPlayerID(playerData.UserId,
						"[tf2stadium.com] You're not in the lobby...")
				}

			case TF2RconWrapper.WorldPlayerDisconnected:
				playerData := message.Parsed.Data.(TF2RconWrapper.PlayerData)
				commID, _ := steamid.SteamIdToCommId(playerData.SteamId)
				if s.IsPlayerAllowed(commID) {
					playerID := helen.GetPlayerID(commID)
					PlayerDisconnected(s.LobbyId, playerID)
				}

			case TF2RconWrapper.ServerCvar:
				varData := message.Parsed.Data.(TF2RconWrapper.CvarData)
				if varData.Variable == "sv_password" {
					// ServerCvar includes the new variable value--but for
					// sv_password it is ***PROTECTED***
					password, err := s.Rcon.GetServerPassword()
					if err == nil && password != s.Info.ServerPassword {
						s.Rcon.ChangeServerPassword(s.Info.ServerPassword)
					}
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

	helpers.Logger.Debug("#%d: Connecting to %s", s.LobbyId, s.Info.Host)

	var err error
	s.Rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
	if err != nil {
		return err
	}

	// kick players
	helpers.Logger.Debug("#%d: Kicking all players", s.LobbyId)
	kickErr := s.KickAll()

	if kickErr != nil {
		return kickErr
	}

	helpers.Logger.Debug("#%d: Setting whitelist", s.LobbyId)
	// whitelist
	_, err = s.Rcon.Query(fmt.Sprintf("tftrue_whitelist_id %s", s.Whitelist))
	if err == TF2RconWrapper.ErrUnknownCommand {
		var whitelist string

		switch {
		case strings.HasPrefix(s.Whitelist, "etf2l_9v9"):
			whitelist = "etf2l_whitelist_9v9.txt"
		case strings.HasPrefix(s.Whitelist, "etf2l_6v6"):
			whitelist = "etf2l_whitelist_6v6.txt"

		case s.Whitelist == "etf2l_ultiduo":
			whitelist = "etf2l_whitelist_ultiduo.txt"

		case s.Whitelist == "etf2l_bball":
			whitelist = "etf2l_whitelist_bball.txt"

		case strings.HasPrefix(s.Whitelist, "ugc_9v9"):
			whitelist = "item_whitelist_ugc_HL.txt"
		case strings.HasPrefix(s.Whitelist, "ugc_6v6"):
			whitelist = "item_whitelist_ugc_6v6.txt"
		case strings.HasPrefix(s.Whitelist, "ugc_4v4"):
			whitelist = "item_whitelist_ugc_4v4.txt"

		case strings.HasPrefix(s.Whitelist, "esea_6v6"):
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

	helpers.Logger.Debug("#%d: Creating listener", s.LobbyId)
	s.ServerListener = helpers.RconListener.CreateServerListener(s.Rcon)
	go s.logListener()

	// change map,
	helpers.Logger.Debug("#%d: Changing Map", s.LobbyId)
	mapErr := s.Rcon.ChangeMap(s.Map)

	if mapErr != nil {
		return mapErr
	}

	helpers.Logger.Debug("#%d: Executing config.", s.LobbyId)
	err = s.ExecConfig()
	if err != nil {
		helpers.Logger.Error(err.Error())
		var count int

		s.Rcon.Close()
		s.Rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
		for err != nil && count != 5 {
			time.Sleep(1 * time.Second)
			helpers.Logger.Critical(err.Error())
			count++
			s.Rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
		}
		if count == 5 {
			helpers.Logger.Error("#%d: %s", s.LobbyId, err.Error())
			return err
		}

		if err = s.ExecConfig(); err != nil {
			helpers.Logger.Error("#%d: %s", s.LobbyId, err.Error())
			return err
		}
		s.Rcon.Query("tftrue_no_hats 0")
	}

	helpers.Logger.Debug("#%d: Configured", s.LobbyId)
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
			DisconnectedFromServer(s.LobbyId)
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
	return helen.IsAllowed(s.LobbyId, commId)
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

func (s *Server) report(data TF2RconWrapper.PlayerData) {
	var team string

	matches := rReport.FindStringSubmatch(data.Text)
	if len(matches) != 3 {
		return
	}

	source, _ := steamid.SteamIdToCommId(data.SteamId)
	team = helen.GetTeam(s.LobbyId, s.Type, source)
	//	helpers.Logger.Debug(team)
	originTeam := team
	if matches[1] == "their" {
		if team == "red" {
			team = "blu"
		} else {
			team = "red"
		}
	} else if matches[1] != "our" {
		s.Rcon.Say("Usage: !rep our/their class")
		return
	}

	target := helen.GetSlotSteamID(team, matches[2], s.LobbyId, s.Type)
	if target == "" {
		s.Rcon.Say("!rep: Unknown or empty slot")
		return
	}

	helpers.Logger.Debug("#%d: %s (team %s) reporting %s (team %s)", s.LobbyId, target, originTeam, target, team)

	err := newReport(source, target, s.LobbyId)
	if err != nil {
		if _, ok := err.(repError); ok {
			s.Rcon.Say("!rep: Already reported")
			helpers.Logger.Error(err.Error())
		} else {
			helpers.Logger.Error(err.Error())
		}
		return
	}

	curReps := countReports(target, s.LobbyId)

	switch curReps {
	case repsNeeded[s.Type]:
		//Got needed number of reports, ask helen to substitute player
		helpers.Logger.Debug("Reported")
		name := helen.GetName(target)

		s.Rcon.Sayf("Reporting %s %s: %s", team, matches[1], name)
		playerID := helen.GetPlayerID(target)
		Substitute(s.LobbyId, playerID)

		resetReportCount(target, s.LobbyId)
		//tell timeout goroutine to stop
		s.StopRepTimer[team+matches[1]] <- struct{}{}

	case 1:
		//first report happened, reset reps one minute later to 0, unless told to stop
		ticker := time.NewTicker(1 * time.Minute)
		stop := make(chan struct{})
		s.StopRepTimer[team+matches[1]] = stop

		go func() {
			select {
			case <-ticker.C:
				s.Rcon.Sayf("Reporting %s %s failed, couldn't get enough !rep in 1 minute.", team, matches[1])
			case <-stop:
				return
			}
		}()

	default:
		helpers.Logger.Debug("Got %d reports", curReps)
	}
	return
}
