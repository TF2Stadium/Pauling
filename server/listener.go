package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TF2Stadium/Helen/models"
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

		s.playerClassesMu.RLock()
		_, ok := s.playerClasses[commID]
		s.playerClassesMu.RUnlock()
		if !ok {
			s.playerClassesMu.Lock()
			s.playerClasses[commID] = &classTime{mu: new(sync.Mutex)}
			s.playerClassesMu.Unlock()
		}

		atomic.AddInt32(s.curplayers, 1)
		if int(atomic.LoadInt32(s.curplayers)) == 2*models.NumberOfClassesMap[s.Type] {
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

var classes = map[string]int{
	"scout":        scout,
	"soldier":      soldier,
	"pyro":         pyro,
	"engineer":     engineer,
	"heavyweapons": heavy,
	"demoman":      demoman,
	"spy":          spy,
	"medic":        medic,
	"sniper":       sniper,
}

type classTime struct {
	mu          *sync.Mutex
	current     int // current class being played
	lastChanged time.Time

	Scout    time.Duration
	Soldier  time.Duration
	Pyro     time.Duration
	Demoman  time.Duration
	Heavy    time.Duration
	Engineer time.Duration
	Sniper   time.Duration
	Medic    time.Duration
	Spy      time.Duration
}

func (s *Server) PlayerSpawned(player TF2RconWrapper.PlayerData, class string) {
	s.PlayerClassChanged(player, class)
}

func (s *Server) PlayerClassChanged(player TF2RconWrapper.PlayerData, class string) {
	commID, _ := steamid.SteamIdToCommId(player.SteamId)

	s.playerClassesMu.RLock()
	classtime, ok := s.playerClasses[commID]
	s.playerClassesMu.RUnlock()

	if !ok { //shouldn't happen, map entry for every player is created when they connect
		helpers.Logger.Errorf("No map entry for %s found", commID)
		return
	}

	classtime.mu.Lock()
	defer classtime.mu.Unlock()

	if classtime.lastChanged.IsZero() {
		classtime.current = classes[class]
		classtime.lastChanged = time.Now()
		return
	}

	var prev *time.Duration // previous class

	switch classtime.current {
	case scout:
		prev = &classtime.Scout
	case soldier:
		prev = &classtime.Soldier
	case pyro:
		prev = &classtime.Pyro
	case demoman:
		prev = &classtime.Demoman
	case heavy:
		prev = &classtime.Heavy
	case engineer:
		prev = &classtime.Engineer
	case sniper:
		prev = &classtime.Sniper
	case medic:
		prev = &classtime.Medic
	case spy:
		prev = &classtime.Spy
	}

	*prev += time.Since(classtime.lastChanged)
	classtime.current = classes[class]
	classtime.lastChanged = time.Now()
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
	if strings.HasPrefix(text, "!rep") {
		s.report(data)
	} else if strings.HasPrefix(text, "!sub") {
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
	} else if strings.HasPrefix(text, "!soapoff ") {
		ExecFile("soap_off.cfg", s.rcon)
	}
}

func (s *Server) GameOver() {
	if atomic.LoadInt32(s.ended) == 1 {
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

	s.playerClassesMu.RLock()
	publishEvent(Event{
		Name:       MatchEnded,
		LobbyID:    s.LobbyId,
		LogsID:     logID,
		ClassTimes: s.playerClasses})
	s.playerClassesMu.RUnlock()

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
