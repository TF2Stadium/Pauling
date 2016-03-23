package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/database"
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/TF2Stadium/Pauling/server/logs"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
	"github.com/TF2Stadium/TF2RconWrapper"
)

const (
	scout = iota
	soldier
	pyro
	demoman
	heavy
	engineer
	sniper
	medic
	spy
)

type classTime struct {
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

type Server struct {
	Map       string // lobby map
	League    string
	Type      models.LobbyType // 9v9 6v6 4v4...
	Whitelist string

	LobbyId uint

	mapMu        *sync.RWMutex
	stopRepTimer map[string]chan struct{}
	StopVerifier chan struct{}

	source *TF2RconWrapper.Source
	rcon   *TF2RconWrapper.TF2RconConnection
	Info   models.ServerRecord

	curplayers int
	// handlers are called synchronously for each server,
	// so we don't need to protect this map
	playerClasses map[string]*classTime //steamID -> playerClasse
}

// func SetupServers() {
// 	helpers.Logger.Debug("Setting up servers")
// 	records := helen.GetServers()
// 	for id, record := range records {
// 		server := NewServer()
// 		server.Info = *record
// 		var count int
// 		var err error

// 		server.Rcon, err = TF2RconWrapper.NewTF2RconConnection(record.Host, record.RconPassword)
// 		for err != nil {
// 			time.Sleep(1 * time.Second)
// 			helpers.Logger.Critical(err.Error())
// 			count++
// 			if count == 5 {
// 				publishEvent(event.Event{
// 					Name:    event.DisconnectedFromServer,
// 					LobbyID: server.LobbyId})
// 				return
// 			}
// 			server.Rcon, err = TF2RconWrapper.NewTF2RconConnection(record.Host, record.RconPassword)
// 		}

// 		SetServer(id, server)

// 		go server.StartVerifier(time.NewTicker(10 * time.Second))

// 		server.Source = listener.AddSourceSecret(record.LogSecret, server, server.Rcon)
// 	}
// }

func NewServer() *Server {
	s := &Server{
		mapMu:         new(sync.RWMutex),
		stopRepTimer:  make(map[string]chan struct{}, 1),
		StopVerifier:  make(chan struct{}, 1),
		playerClasses: make(map[string]*classTime),
	}

	return s
}

func (s *Server) StopListening() {
	Listener.RemoveSource(s.source, s.rcon)
	s.StopVerifier <- struct{}{}
}

func (s *Server) GetPlayers() ([]TF2RconWrapper.Player, error) {
	return s.rcon.GetPlayers()
}

func (s *Server) KickPlayer(commID string, reason string) error {
	steamID, _ := steamid.CommIdToSteamId(commID) //convert community id to steam id

	players, err := s.rcon.GetPlayers()
	if err != nil {
		helpers.Logger.Errorf("%v", err)
	}

	for _, player := range players {
		if steamid.SteamIDsEqual(steamID, player.SteamID) {
			return s.rcon.KickPlayer(player, reason)
		}
	}

	return nil
}

func (s *Server) Say(text string) error {
	return s.rcon.Say(text)
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
	defer DeleteServer(s.LobbyId)

	_, err = s.rcon.Query("status")
	if err != nil {
		err = s.rcon.Reconnect(5 * time.Minute)

		if err != nil {
			publishEvent(Event{
				Name:    DisconnectedFromServer,
				LobbyID: s.LobbyId})
			return
		}
	}

	for {
		select {
		case <-ticker.C:
			if !s.Verify() {
				ticker.Stop()
				s.rcon.Close()
				return
			}
		case <-s.StopVerifier:
			helpers.Logger.Debugf("Stopping logger for lobby %d", s.LobbyId)
			s.rcon.Say("[tf2stadium.com] Lobby Ended.")
			s.rcon.RemoveTag("TF2Stadium")
			ticker.Stop()
			s.rcon.Close()
			return
		}
	}
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
		s.curplayers++
		if s.curplayers == 2*models.NumberOfClassesMap[s.Type] {
			ExecFile("soap_off.cfg", s.rcon)
		}
		_, ok := s.playerClasses[commID]
		if !ok {
			s.playerClasses[commID] = new(classTime)
		}
	} else {
		s.rcon.KickPlayerID(data.UserId, "[tf2stadium.com] "+reason)
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

func (s *Server) PlayerClassChanged(player TF2RconWrapper.PlayerData, class string) {
	commID, _ := steamid.SteamIdToCommId(player.SteamId)
	classtime, ok := s.playerClasses[commID]
	if !ok { //shouldn't happen, map entry for every player is created when they connect
		helpers.Logger.Errorf("No map entry for %s found", commID)
		return
	}

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
	//Logger.Debug("GLOBAL %s:", text)

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
	s.StopListening()
	logsBuff := s.source.Logs()
	logsBuff.WriteString("L " + time.Now().Format(TF2RconWrapper.TimeFormat) + ": Log file closed.\n")

	if config.Constants.LogsTFAPIKey == "" {
		helpers.Logger.Debug("No logs.tf API key, writing logs to file")
		ioutil.WriteFile(fmt.Sprintf("%d.log", s.LobbyId), logsBuff.Bytes(), 0666)
	}

	logID, err := logs.Upload(fmt.Sprintf("TF2Stadium Lobby #%d", s.LobbyId), s.Map, logsBuff)
	if err != nil {
		helpers.Logger.Warningf("%d: %s", s.LobbyId, err.Error())
		ioutil.WriteFile(fmt.Sprintf("%d.log", s.LobbyId), logsBuff.Bytes(), 0666)
	}
	publishEvent(Event{
		Name:       MatchEnded,
		LobbyID:    s.LobbyId,
		LogsID:     logID,
		ClassTimes: s.playerClasses})
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

func (s *Server) Setup() error {
	helpers.Logger.Debugf("#%d: Connecting to %s", s.LobbyId, s.Info.Host)

	var err error
	s.rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
	if err != nil {
		return err
	}

	// kick players
	helpers.Logger.Debugf("#%d: Kicking all players", s.LobbyId)
	kickErr := s.KickAll()

	if kickErr != nil {
		return kickErr
	}

	helpers.Logger.Debugf("#%d: Setting whitelist", s.LobbyId)
	// whitelist
	_, err = s.rcon.Query(fmt.Sprintf("tftrue_whitelist_id %s", s.Whitelist))
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
		s.rcon.Query("mp_tournament_whitelist " + whitelist)
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

	helpers.Logger.Debugf("#%d: Creating listener", s.LobbyId)
	eventlistener := &TF2RconWrapper.EventListener{
		PlayerConnected:     s.PlayerConnected,
		PlayerDisconnected:  s.PlayerDisconnected,
		PlayerGlobalMessage: s.PlayerGlobalMessage,
		GameOver:            s.GameOver,
		CVarChange:          s.CVarChange,
		PlayerClassChanged:  s.PlayerClassChanged,
		TournamentStarted:   s.TournamentStarted,
	}

	s.source = Listener.AddSource(eventlistener, s.rcon)
	database.SetSecret(s.source.Secret, s.Info.ID)

	// change map,
	helpers.Logger.Debugf("#%d: Changing Map", s.LobbyId)
	mapErr := s.rcon.ChangeMap(s.Map)

	if mapErr != nil {
		return mapErr
	}

	err = s.rcon.Reconnect(2 * time.Minute)
	if err != nil {
		return err
	}
	s.rcon.AddTag("TF2Stadium")
	helpers.Logger.Debugf("#%d: Executing config.", s.LobbyId)
	err = s.execConfig()
	if err != nil {
		s.rcon.RemoveTag("TF2Stadium")
		return err
	}

	s.rcon.Query("tftrue_no_hats 0")

	helpers.Logger.Debugf("#%d: Configured", s.LobbyId)
	return nil
}

func (s *Server) Reset() {
	s.rcon.ChangeMap(s.Map)
	s.rcon.Reconnect(2 * time.Minute)
	s.execConfig()
}

func (s *Server) execConfig() error {
	var err error

	leagueConfigPath, err := ConfigName(s.Map, s.Type, s.League)
	if err != nil {
		return err
	}

	formatConfigPath := FormatConfigName(s.Type)

	if s.Type != models.LobbyTypeDebug {
		err = ExecFile("base.cfg", s.rcon)
		if err != nil {
			return err
		}
	}

	err = ExecFile(formatConfigPath, s.rcon)
	if err != nil {
		return err
	}
	err = ExecFile(leagueConfigPath, s.rcon)

	if s.Type != models.LobbyTypeDebug {
		err = ExecFile("after_format.cfg", s.rcon)
		if err != nil {
			return err
		}
	}

	return err
}

// runs each 10 sec
func (s *Server) Verify() bool {
	//Logger.Debug("#%d: Verifying %s...", s.LobbyId, s.Info.Host)
	password, err := s.rcon.GetServerPassword()
	if err == nil {
		if password == s.Info.ServerPassword {
			return true
		}
	}

	err = s.rcon.ChangeServerPassword(s.Info.ServerPassword)
	if err != nil {
		err = s.rcon.Reconnect(5 * time.Minute)
		if err != nil {
			publishEvent(Event{
				Name:    DisconnectedFromServer,
				LobbyID: s.LobbyId})
		}

	}

	players, _ := s.rcon.GetPlayers()
	publishEvent(Event{
		Name:    "playersList",
		Players: players,
	})

	return true
}

func (s *Server) KickAll() error {
	_, err := s.rcon.Query("kickall")

	return err
}

var (
	rReport        = regexp.MustCompile(`^!rep\s+(.+)\s+(.+)`)
	rFirstSubArg   = regexp.MustCompile(`^!sub\s+(.+)`)
	stopRepTimeout = make(map[uint](chan struct{}))

	repsNeeded = map[models.LobbyType]int{
		models.LobbyTypeSixes:      5,
		models.LobbyTypeDebug:      2,
		models.LobbyTypeHighlander: 6,
		models.LobbyTypeFours:      4,
		models.LobbyTypeBball:      3,
		models.LobbyTypeUltiduo:    3,
	}
)

func (s *Server) report(data TF2RconWrapper.PlayerData) {
	var team string

	matches := rReport.FindStringSubmatch(data.Text)
	if len(matches) != 3 {
		s.rcon.Say("Usage: !rep our/their/red/blu slotname")
		return
	}

	argTeam := strings.ToLower(matches[1])
	argSlot := strings.ToLower(matches[2])

	source, _ := steamid.SteamIdToCommId(data.SteamId)
	if database.IsReported(s.LobbyId, source) {
		s.rcon.Say("!rep: Player has already been reported.")
		return
	}

	team = database.GetTeam(s.LobbyId, s.Type, source)
	//	helpers.Logger.Debug(team)

	switch argTeam {
	case "their":
		if team == "red" {
			team = "blu"
		} else {
			team = "red"
		}
	case "our":
		// team = team
	case "blu", "red":
		team = argTeam
	case "blue":
		team = "blu"
	default:
		s.rcon.Say("Usage: !rep our/their/red/blu slotname")
		return
	}

	target, err := database.GetSteamIDFromSlot(team, argSlot, s.LobbyId, s.Type)
	if err != nil {
		s.rcon.Say("!rep: Unknown or empty slot")
		return
	}

	if database.IsReported(s.LobbyId, target) {
		s.rcon.Say("Player has already been reported")
		return
	}

	if target == source {
		// !rep'ing themselves
		publishEvent(Event{
			Name:    PlayerSubstituted,
			LobbyID: s.LobbyId,
			SteamID: source,
			Self:    true})

		say := fmt.Sprintf("Reporting player %s (%s)", data.Username, data.SteamId)
		s.rcon.Say(say)
		return
	}

	err = newReport(source, target, s.LobbyId)

	if err != nil {
		if _, ok := err.(*repError); ok {
			s.rcon.Say("!rep: Already reported")
		} else {
			s.rcon.Say(err.Error())
			helpers.Logger.Errorf("#%d: %v", s.LobbyId, err)
		}
		return
	}

	curReps := countReports(target, s.LobbyId)
	name := database.GetNameFromSteamID(target)

	say := fmt.Sprintf("Got %d votes votes for reporting %s (%d needed)", curReps, name, repsNeeded[s.Type])
	s.rcon.Say(say)

	switch curReps {
	case repsNeeded[s.Type]:
		//Got needed number of reports, ask helen to substitute player
		say := fmt.Sprintf("Reporting %s %s: %s", team, argSlot, name)
		s.rcon.Say(say)
		publishEvent(Event{
			Name:    PlayerSubstituted,
			SteamID: target,
			LobbyID: s.LobbyId})

		// tell timeout goroutine to stop (It is possible that the map
		// entry will not exist if only 1 report is needed (such as debug
		// lobbies))
		s.mapMu.RLock()
		c, ok := s.stopRepTimer[team+argSlot]
		s.mapMu.RUnlock()

		if ok {
			c <- struct{}{}
		}

	case 1:
		//first report happened, reset reps one minute later to 0, unless told to stop
		ticker := time.NewTicker(2 * time.Minute)
		stop := make(chan struct{})

		s.mapMu.Lock()
		s.stopRepTimer[team+argSlot] = stop
		s.mapMu.Unlock()

		go func() {
			select {
			case <-ticker.C:
				say := fmt.Sprintf("Reporting %s %s failed, couldn't get enough votes in 2 minute.", strings.ToUpper(team), strings.ToUpper(argSlot))
				s.rcon.Say(say)
				ResetReportCount(target, s.LobbyId)
			case <-stop:
				return
			}
			//once a sub is found, the report count will be reset with
			//the rpc's DisallowPlayer method
			s.mapMu.Lock()
			delete(s.stopRepTimer, team+argSlot)
			s.mapMu.Unlock()
		}()
	}

	return
}
