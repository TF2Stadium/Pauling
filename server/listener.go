package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TF2Stadium/Helen/models/lobby/format"
	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/database"
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/TF2Stadium/Pauling/server/logs"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
	"github.com/TF2Stadium/TF2RconWrapper"
)

var (
	Listener   *TF2RconWrapper.Listener
	externalIP = getlocalip()
)

func getlocalip() string {
	resp, err := http.Get("http://api.ipify.org")
	if err != nil {
		helpers.Logger.Fatal(err)
	}
	bytes, _ := ioutil.ReadAll(resp.Body)
	return string(bytes)
}

func StartListener() {
	var err error

	Listener, err = TF2RconWrapper.NewListenerAddr(config.Constants.LogsPort, externalIP+":"+config.Constants.LogsPort, config.Constants.PrintLogMessages)

	if err != nil {
		helpers.Logger.Fatal(err)
	}

	helpers.Logger.Info("Listening for server messages on %s:%s", externalIP, config.Constants.LogsPort)

	connectMQ()
}

func (s *Server) PlayerConnected(data TF2RconWrapper.PlayerData) {
	commID, _ := steamid.SteamIdToCommId(data.SteamId)
	allowed, reason := database.IsAllowed(s.LobbyId, commID)
	if allowed {
		publishEvent(Event{
			Name:    PlayerConnected,
			LobbyID: s.LobbyId,
			SteamID: commID,
		})

		atomic.AddInt32(s.curplayers, 1)
		if int(atomic.LoadInt32(s.curplayers)) == 2*format.NumberOfClassesMap[s.Type] {
			ExecFile("soap_off.cfg", s.rcon)
		}
	} else {
		s.rcon.KickPlayerID(data.UserId, "[tf2stadium.com] "+reason)
	}
}

func (s *Server) RconCommand(_, command string) {
	if strings.Contains(command, `Reservation ended, every player can download the STV demo at`) {
		publishEvent(Event{
			Name:    ReservationOver,
			LobbyID: s.LobbyId})

		s.StopListening()
	}
}

func (s *Server) PlayerDisconnected(data TF2RconWrapper.PlayerData) {
	commID, _ := steamid.SteamIdToCommId(data.SteamId)
	allowed, _ := database.IsAllowed(s.LobbyId, commID)
	if allowed {
		publishEvent(Event{
			Name:    PlayerDisconnected,
			LobbyID: s.LobbyId,
			SteamID: commID})
	}
}

func (s *Server) TournamentStarted() {
	ExecFile("soap_off.cfg", s.rcon)
}

func (s *Server) PlayerGlobalMessage(data TF2RconWrapper.PlayerData, text string) {
	switch {
	case strings.HasPrefix(text, "!rep"):
		s.report(data)
	case strings.HasPrefix(text, "!sub"):
		if rFirstSubArg.FindStringSubmatch(text) != nil {
			// If they tried to use !sub with an argument, they
			// probably meant to !rep
			s.rcon.Say("!sub is for replacing yourself, !rep reports others.")
		} else {
			commID, _ := steamid.SteamIdToCommId(data.SteamId)

			publishEvent(Event{
				Name:    PlayerSubstituted,
				LobbyID: s.LobbyId,
				SteamID: commID,
				Self:    true})

			say := fmt.Sprintf("Reporting player %s (%s)",
				data.Username, data.SteamId)
			s.rcon.Say(say)
		}
	case strings.HasPrefix(text, "!soapoff"):
		ExecFile("soap_off.cfg", s.rcon)
	case strings.HasPrefix(text, "!help"):
		s.rcon.Say(`Use !rep for reporting, !sub for substituting yourself.`)
	case strings.HasPrefix(text, "!kick"):

	}
}

func (s *Server) GameOver() {
	if s.hasEnded() {
		return
	}
	atomic.StoreInt32(s.ended, 1)

	logsBuff := s.source.Logs()
	logsBuff.WriteString("L " + time.Now().Format(TF2RconWrapper.TimeFormat) + ": Log file closed.\n")

	var logID int
	if config.Constants.LogsTFAPIKey != "" {
		var err error

		logID, err = logs.Upload(fmt.Sprintf("TF2Stadium Lobby #%d", s.LobbyId), s.Map, logsBuff)
		if err != nil {
			helpers.Logger.Warningf("%d: %s", s.LobbyId, err.Error())
			ioutil.WriteFile(fmt.Sprintf("%d.log", s.LobbyId), logsBuff.Bytes(), 0666)
		}
	} else {
		helpers.Logger.Debug("No logs.tf API key, writing logs to file")
		ioutil.WriteFile(fmt.Sprintf("%d.log", s.LobbyId), logsBuff.Bytes(), 0666)
	}

	publishEvent(Event{
		Name:    MatchEnded,
		LobbyID: s.LobbyId,
		LogsID:  logID})

	s.StopListening()
	return
}

func (s *Server) CVarChange(variable string, value string) {
	if variable == "sv_password" {
		// ServerCvar includes the new variable value--but for
		// sv_password it is ***PROTECTED***
		if value != s.Info.ServerPassword {
			s.rcon.ChangeServerPassword(s.Info.ServerPassword)
		}
	}
}
