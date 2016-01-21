package server

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/TF2Stadium/Helen/config"
	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/Pauling/helen"
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/TF2Stadium/Pauling/logs"
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
	Rcon           *TF2RconWrapper.TF2RconConnection
	Info           models.ServerRecord
	logs           *bytes.Buffer

	matchEnded bool
}

func NewServer() *Server {
	s := &Server{
		StopRepTimer:    make(map[string]chan struct{}),
		StopVerifier:    make(chan struct{}),
		StopLogListener: make(chan struct{}),
		logs:            new(bytes.Buffer),
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
	defer DeleteServer(s.LobbyId)

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
				return
			}
		case <-s.StopVerifier:
			helpers.Logger.Debug("Stopping logger for lobby %d", s.LobbyId)
			s.Rcon.Say("[tf2stadium.com] Lobby Ended.")
			ticker.Stop()
			s.Rcon.Close()
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
			rawMessage := message.Message[:len(message.Message)-2]

			if helpers.PrintLogMessages {
				helpers.Logger.Debug("L " + rawMessage)
			}
			//helpers.Logger.Debug(message.Message)

			if err != nil {
				continue
			}
			s.logs.WriteString("L " + rawMessage + "\n")

			switch message.Parsed.Type {
			case TF2RconWrapper.WorldGameOver:
				s.matchEnded = true

			case TF2RconWrapper.LogFileClosed:
				if !s.matchEnded {
					continue
				}
				//log has ended, try uploading
				go s.ServerListener.Close(s.Rcon)
				s.StopVerifier <- struct{}{}
				logID, err := logs.Upload(fmt.Sprintf("TF2Stadium Lobby #%d", s.LobbyId), s.Map, s.logs)
				if err != nil {
					helpers.Logger.Warning("%d: %s", s.LobbyId, err.Error())
					ioutil.WriteFile(fmt.Sprintf("%d.log", s.LobbyId), s.logs.Bytes(), 0666)
				}
				MatchEnded(s.LobbyId, logID)
				return

			case TF2RconWrapper.PlayerGlobalMessage:
				playerData := message.Parsed.Data.(TF2RconWrapper.PlayerData)
				text := playerData.Text
				//Logger.Debug("GLOBAL %s:", text)

				if strings.HasPrefix(text, "!rep") {
					s.report(playerData)
				} else if strings.HasPrefix(text, "!sub") {
					if rFirstSubArg.FindStringSubmatch(text) != nil {
						// If they tried to use !sub with an argument, they
						// probably meant to !rep
						s.Rcon.Say("!sub is for replacing yourself, !rep reports others.")
					} else {
						commID, _ := steamid.SteamIdToCommId(playerData.SteamId)
						playerID := helen.GetPlayerID(commID)

						Substitute(s.LobbyId, playerID)

						say := fmt.Sprintf("Reporting player %s (%s)",
							playerData.Username, playerData.SteamId)
						s.Rcon.Say(say)
					}
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
	rFirstSubArg   = regexp.MustCompile(`^!sub\s+(.+)`)
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

func (s *Server) repUsage() {
	s.Rcon.Say("Usage: !rep our/their/red/blu slotname")
}

func (s *Server) report(data TF2RconWrapper.PlayerData) {
	var team string

	matches := rReport.FindStringSubmatch(data.Text)
	if len(matches) != 3 {
		s.repUsage()
		return
	}

	argTeam := strings.ToLower(matches[1])
	argSlot := strings.ToLower(matches[2])

	source, _ := steamid.SteamIdToCommId(data.SteamId)
	team = helen.GetTeam(s.LobbyId, s.Type, source)
	//	helpers.Logger.Debug(team)
	originTeam := team
	if argTeam == "their" {
		if team == "red" {
			team = "blu"
		} else {
			team = "red"
		}
	} else if argTeam == "blu" || argTeam == "red" {
		team = argTeam
	} else if argTeam != "our" {
		s.repUsage()
		return
	}

	target := helen.GetSlotSteamID(team, argSlot, s.LobbyId, s.Type)
	if target == "" {
		s.Rcon.Say("!rep: Unknown or empty slot")
		return
	}

	helpers.Logger.Debug("#%d: %s (team %s) reporting %s (team %s)", s.LobbyId, source, originTeam, target, team)

	if target == source {
		// !rep'ing themselves
		playerID := helen.GetPlayerID(source)
		Substitute(s.LobbyId, playerID)

		helpers.Logger.Debug("repported target == source")
		say := fmt.Sprintf("Reporting player %s (%s)", data.Username, data.SteamId)
		s.Rcon.Say(say)
		return
	}

	err := newReport(source, target, s.LobbyId)
	if err != nil {
		if _, ok := err.(repError); ok {
			s.Rcon.Say("!rep: Already reported")
			helpers.Logger.Error(err.Error())
		} else {
			s.Rcon.Say("!rep: Reporting system error")
			helpers.Logger.Error(err.Error())
		}
		return
	}

	curReps := countReports(target, s.LobbyId)

	name := helen.GetName(target)
	switch curReps {
	case repsNeeded[s.Type]:
		//Got needed number of reports, ask helen to substitute player
		helpers.Logger.Debug("Reported")

		s.Rcon.Sayf("Reporting %s %s: %s", team, argSlot, name)
		playerID := helen.GetPlayerID(target)
		Substitute(s.LobbyId, playerID)

		// tell timeout goroutine to stop (It is possible that the map
		// entry will not exist if only 1 report is needed (such as debug
		// lobbies))
		if c, ok := s.StopRepTimer[team+argSlot]; ok {
			c <- struct{}{}
		}

	case 1:
		//first report happened, reset reps one minute later to 0, unless told to stop
		ticker := time.NewTicker(1 * time.Minute)
		stop := make(chan struct{})
		s.StopRepTimer[team+argSlot] = stop

		go func() {
			select {
			case <-ticker.C:
				s.Rcon.Sayf("Reporting %s %s failed, couldn't get enough !rep in 1 minute.", team, argSlot)
			case <-stop:
				return
			}
			delete(s.StopRepTimer, team+argSlot)
		}()

	default:
		s.Rcon.Sayf("Got %d votes votes for reporting player %s (%d needed)", curReps, name, repsNeeded[s.Type])
		helpers.Logger.Debug("Got %d reports", curReps)
	}
	return
}
