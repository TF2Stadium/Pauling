package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

	StopRepTimer map[string]chan struct{}
	StopVerifier chan struct{}

	source *TF2RconWrapper.Source
	rcon   *TF2RconWrapper.TF2RconConnection
	Info   models.ServerRecord

	matchEnded bool
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
		StopRepTimer:  make(map[string]chan struct{}),
		StopVerifier:  make(chan struct{}),
		playerClasses: make(map[string]*classTime),
	}

	return s
}

func (s *Server) StopListening() {
	go listener.RemoveSource(s.source, s.rcon)
	s.StopVerifier <- struct{}{}
}

func (s *Server) GetPlayers() ([]TF2RconWrapper.Player, error) {
	return s.rcon.GetPlayers()
}

func (s *Server) KickPlayer(commID string, reason string) {
	steamID, _ := steamid.CommIdToSteamId(commID) //legacy steam id

	players, _ := s.rcon.GetPlayers()
	for _, player := range players {
		if player.SteamID == steamID {
			s.rcon.KickPlayer(player, reason)
		}
	}
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
	var count int
	defer DeleteServer(s.LobbyId)

	_, err = s.rcon.Query("status")
	if err != nil {
		s.rcon.Close()
		s.rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
		for err != nil && count != 5 {
			time.Sleep(1 * time.Second)
			helpers.Logger.Critical(err.Error())
			count++
			s.rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
		}
		if count == 5 {
			publishEvent(Event{
				Name:    DisconnectedFromServer,
				LobbyID: s.LobbyId})

			listener.RemoveSource(s.source, s.rcon)
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
			helpers.Logger.Debug("Stopping logger for lobby %d", s.LobbyId)
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
				SteamID: commID})

			say := fmt.Sprintf("Reporting player %s (%s)",
				data.Username, data.SteamId)
			s.rcon.Say(say)
		}
	} else if strings.HasPrefix(text, "!soapoff ") {
		ExecFile("soap_off.cfg", s.rcon)
	}
}

func (s *Server) PlayerTeamMessage(TF2RconWrapper.PlayerData, string) {}

func (s *Server) PlayerClassChange(TF2RconWrapper.PlayerData, string) {}

func (s *Server) PlayerTeamChange(TF2RconWrapper.PlayerData, string) {}

func (s *Server) GameOver() {
	s.matchEnded = true
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

func (s *Server) LogFileClosed() {
	if !s.matchEnded {
		return
	}

	logsBuff := s.source.Logs()
	s.StopListening()

	if config.Constants.LogsTFAPIKey == "" {
		helpers.Logger.Debug("No logs.tf API key, writing logs to file")
		ioutil.WriteFile(fmt.Sprintf("%d.log", s.LobbyId), logsBuff.Bytes(), 0666)
	}

	logID, err := logs.Upload(fmt.Sprintf("TF2Stadium Lobby #%d", s.LobbyId), s.Map, logsBuff)
	if err != nil {
		helpers.Logger.Warning("%d: %s", s.LobbyId, err.Error())
		ioutil.WriteFile(fmt.Sprintf("%d.log", s.LobbyId), logsBuff.Bytes(), 0666)
	}
	publishEvent(Event{
		Name:       MatchEnded,
		LobbyID:    s.LobbyId,
		LogsID:     logID,
		ClassTimes: s.playerClasses})
	return
}

func (s *Server) Setup() error {
	helpers.Logger.Debug("#%d: Connecting to %s", s.LobbyId, s.Info.Host)

	var err error
	s.rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
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

	helpers.Logger.Debug("#%d: Creating listener", s.LobbyId)
	eventlistener := &TF2RconWrapper.EventListener{
		PlayerConnected:     s.PlayerConnected,
		PlayerDisconnected:  s.PlayerDisconnected,
		PlayerGlobalMessage: s.PlayerGlobalMessage,
		GameOver:            s.GameOver,
		CVarChange:          s.CVarChange,
		LogFileClosed:       s.LogFileClosed,
		PlayerClassChanged:  s.PlayerClassChanged,
		TournamentStarted:   s.TournamentStarted,
	}

	s.source = listener.AddSource(eventlistener, s.rcon)
	database.SetSecret(s.source.Secret, s.Info.ID)

	// change map,
	helpers.Logger.Debug("#%d: Changing Map", s.LobbyId)
	mapErr := s.rcon.ChangeMap(s.Map)

	if mapErr != nil {
		return mapErr
	}

	time.Sleep(5 * time.Second)
	helpers.Logger.Debug("#%d: Executing config.", s.LobbyId)
	s.rcon.AddTag("TF2Stadium")
	err = s.ExecConfig()
	if err != nil { // retry connection
		var count int
		helpers.Logger.Error("%v", err)

		s.rcon.Close()
		s.rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
		for err != nil && count != 5 {
			time.Sleep(1 * time.Second)
			count++
			s.rcon, err = TF2RconWrapper.NewTF2RconConnection(s.Info.Host, s.Info.RconPassword)
		}
		if count == 5 {
			helpers.Logger.Error("#%d: %s", s.LobbyId, err.Error())
			return err
		}

		s.ExecConfig()
	}

	s.rcon.Query("tftrue_no_hats 0")

	helpers.Logger.Debug("#%d: Configured", s.LobbyId)
	return nil
}

func (s *Server) ExecConfig() error {
	var err error

	configPath, err := ConfigName(s.Map, s.Type, s.League)
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

	err = ExecFile(configPath, s.rcon)

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
	retries := 0
	for err != nil { //TODO: Stop connection after x retries
		if retries == 6 {
			//Logger.Warning("#%d: Couldn't query %s after 5 retries", s.LobbyId, s.Info.Host)
			publishEvent(Event{
				Name:    DisconnectedFromServer,
				LobbyID: s.LobbyId})

			return false
		}
		retries++
		time.Sleep(time.Second)
		err = s.rcon.ChangeServerPassword(s.Info.ServerPassword)
	}

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
		models.LobbyTypeSixes:      7,
		models.LobbyTypeDebug:      1,
		models.LobbyTypeHighlander: 7,
		models.LobbyTypeFours:      5,
		models.LobbyTypeBball:      3,
		models.LobbyTypeUltiduo:    3,
	}
)

func (s *Server) repUsage() {
	s.rcon.Say("Usage: !rep our/their/red/blu slotname")
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
	if database.IsReported(s.LobbyId, source) {
		s.rcon.Say("!rep: Player has already been reported.")
		return
	}
	team = database.GetTeam(s.LobbyId, s.Type, source)
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

	target := database.GetSteamIDFromSlot(team, argSlot, s.LobbyId, s.Type)
	if target == "" {
		s.rcon.Say("!rep: Unknown or empty slot")
		return
	}

	helpers.Logger.Debug("#%d: %s (team %s) reporting %s (team %s)", s.LobbyId, source, originTeam, target, team)

	if target == source {
		// !rep'ing themselves
		publishEvent(Event{
			Name:    PlayerSubstituted,
			LobbyID: s.LobbyId,
			SteamID: source})

		helpers.Logger.Debug("repported target == source")
		say := fmt.Sprintf("Reporting player %s (%s)", data.Username, data.SteamId)
		s.rcon.Say(say)
		return
	}

	err := newReport(source, target, s.LobbyId)
	if err != nil {
		if _, ok := err.(repError); ok {
			s.rcon.Say("!rep: Already reported")
			helpers.Logger.Error(err.Error())
		} else {
			s.rcon.Say("!rep: Reporting system error")
			helpers.Logger.Error(err.Error())
		}
		return
	}

	curReps := countReports(target, s.LobbyId)

	name := database.GetNameFromSteamID(target)
	switch curReps {
	case repsNeeded[s.Type]:
		//Got needed number of reports, ask helen to substitute player
		helpers.Logger.Debug("Reported")

		s.rcon.Sayf("Reporting %s %s: %s", team, argSlot, name)
		publishEvent(Event{
			Name:    PlayerSubstituted,
			SteamID: target,
			LobbyID: s.LobbyId})

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
				s.rcon.Sayf("Reporting %s %s failed, couldn't get enough !rep in 1 minute.", team, argSlot)
			case <-stop:
				return
			}
			delete(s.StopRepTimer, team+argSlot)
		}()

	default:
		s.rcon.Sayf("Got %d votes votes for reporting player %s (%d needed)", curReps, name, repsNeeded[s.Type])
		helpers.Logger.Debug("Got %d reports", curReps)
	}
	return
}
