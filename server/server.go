package server

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TF2Stadium/Helen/models/gameserver"
	"github.com/TF2Stadium/Helen/models/lobby/format"
	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/database"
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
	"github.com/TF2Stadium/TF2RconWrapper"
)

type Server struct {
	Map       string // lobby map
	League    string
	Type      format.Format // 9v9 6v6 4v4...
	Whitelist string

	LobbyId uint

	mapMu        sync.RWMutex
	repTimer     map[string]*time.Timer
	StopVerifier chan struct{}

	source *TF2RconWrapper.Source
	rcon   *TF2RconWrapper.TF2RconConnection
	Info   gameserver.ServerRecord

	curplayers *int32
	ended      *int32
}

func NewServer() *Server {
	s := &Server{
		repTimer:     make(map[string]*time.Timer),
		StopVerifier: make(chan struct{}, 1),
		curplayers:   new(int32),
		ended:        new(int32),
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

	s.rcon.Query("mp_tournament 1; mp_tournament_readymode 1")
	// change map,
	helpers.Logger.Debugf("#%d: Changing Map", s.LobbyId)
	err = s.rcon.ChangeMap(s.Map)

	if err != nil {
		return err
	}

	time.Sleep(10 * time.Second) // wait for map to change

	err = s.rcon.Reconnect(2 * time.Minute)
	if err != nil {
		return err
	}

	helpers.Logger.Debugf("#%d: Creating listener", s.LobbyId)
	eventlistener := &TF2RconWrapper.EventListener{
		PlayerConnected:     s.PlayerConnected,
		PlayerDisconnected:  s.PlayerDisconnected,
		PlayerGlobalMessage: s.PlayerGlobalMessage,
		GameOver:            s.GameOver,
		CVarChange:          s.CVarChange,
		TournamentStarted:   s.TournamentStarted,
		RconCommand:         s.RconCommand,
	}

	s.source = Listener.AddSource(eventlistener, s.rcon)
	database.SetSecret(s.source.Secret, s.Info.ID)

	s.rcon.AddTag("TF2Stadium")
	s.rcon.Query("tftrue_no_hats 0; mp_timelimit 0; mp_tournament 1; mp_tournament_restart")

	helpers.Logger.Debugf("#%d: Setting whitelist", s.LobbyId)
	s.execWhitelist()

	// Yes, we do not execute the config here.
	// The config is executed a bit after the server is configured,
	// hwen Helen makes the ReExecConfig RPC call.
	helpers.Logger.Debugf("#%d: Configured", s.LobbyId)
	return nil
}

func (s *Server) execWhitelist() {
	// whitelist
	_, err := s.rcon.Query(fmt.Sprintf("tftrue_whitelist_id %s", s.Whitelist))
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

}

func (s *Server) Reset(changeMap bool) {
	if changeMap {
		s.rcon.ChangeMap(s.Map)
		s.rcon.Reconnect(2 * time.Minute)
	}
	s.execConfig()
}

func (s *Server) execConfig() error {
	var err error

	leagueConfigPath, err := ConfigName(s.Map, s.Type, s.League)
	if err != nil {
		return err
	}

	if s.Type != format.Debug {
		err = ExecFile("base.cfg", s.rcon)
		if err != nil {
			return err
		}
	}

	formatConfigPath := FormatConfigName(s.Type)
	err = ExecFile(formatConfigPath, s.rcon)
	if err != nil {
		return err
	}
	err = ExecFile(leagueConfigPath, s.rcon)

	if s.Type != format.Debug {
		err = ExecFile("after_format.cfg", s.rcon)
		if err != nil {
			return err
		}
	}

	s.execWhitelist()
	return err
}

// runs each 10 sec
func (s *Server) Verify() bool {
	//Logger.Debug("#%d: Verifying %s...", s.LobbyId, s.Info.Host)
	password, err := s.rcon.GetServerPassword()

	if err == nil {
		players, _ := s.rcon.GetPlayers()
		publishEvent(Event{
			Name:    "playersList",
			Players: players,
		})

		s.rcon.QueryNoResp("sv_logsecret " + s.source.Secret + "; logaddress_add " + externalIP + ":" + config.Constants.LogsPort)

		if password == s.Info.ServerPassword {
			return true
		}
	}

	whitelist, _ := s.rcon.GetConVar("tftrue_whitelist_id")
	if whitelist != s.Whitelist {
		s.execWhitelist()
	}

	err = s.rcon.ChangeServerPassword(s.Info.ServerPassword)
	if err != nil {
		err = s.rcon.Reconnect(5 * time.Minute)
		if err != nil && !s.hasEnded() {
			publishEvent(Event{
				Name:    DisconnectedFromServer,
				LobbyID: s.LobbyId})
		}

	}

	return true
}

func (s *Server) hasEnded() bool {
	return atomic.LoadInt32(s.ended) == 1
}

func (s *Server) KickAll() error {
	_, err := s.rcon.Query("kickall")

	return err
}

var (
	rReport      = regexp.MustCompile(`^!rep\s+(.+)\s+(.+).*`)
	rFirstSubArg = regexp.MustCompile(`^!sub\s+(.+)`)

	repsNeeded = map[format.Format]int{
		format.Sixes:      5,
		format.Debug:      2,
		format.Highlander: 6,
		format.Fours:      4,
		format.Bball:      3,
		format.Ultiduo:    3,
	}
)

func slot(f format.Format) string {
	switch f {
	case format.Sixes:
		return "scout1/scout2/demoman/pocket/roamer/medic"
	case format.Highlander:
		return "scout/soldier/pyro/demoman/heavy/engineer/medic/spy/sniper"
	case format.Ultiduo:
		return "soldier/medic"
	case format.Bball:
		return "soldier1/soldier2"
	default:
		return "class name"
	}

}

func (s *Server) report(data TF2RconWrapper.PlayerData) {
	var team string

	matches := rReport.FindStringSubmatch(data.Text)
	if len(matches) != 3 {
		s.rcon.Say("Usage: !rep our/their/red/blu " + slot(s.Type))
		return
	}

	argTeam := strings.ToLower(matches[1])
	argSlot := strings.ToLower(matches[2])

	source, _ := steamid.SteamIdToCommId(data.SteamId)

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
		var slots string

		s.rcon.Say("!rep: valid slots - " + slots)
		return
	}

	if database.IsReported(s.LobbyId, target) {
		s.rcon.Say("!rep: Player has already been reported")
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
			s.rcon.Say("!rep: You have already voted.")
		} else {
			s.rcon.Say(err.Error())
			helpers.Logger.Errorf("#%d: %v", s.LobbyId, err)
		}
		return
	}

	curReps := countReports(target, s.LobbyId)
	name := database.GetNameFromSteamID(target)

	switch curReps {
	case repsNeeded[s.Type]:
		//Got needed number of reports, ask helen to substitute player
		publishEvent(Event{
			Name:    PlayerSubstituted,
			SteamID: target,
			LobbyID: s.LobbyId})

		// tell timeout goroutine to stop (It is possible that the map
		// entry will not exist if only 1 report is needed (such as debug
		// lobbies))
		s.mapMu.RLock()
		timer, ok := s.repTimer[team+argSlot]
		s.mapMu.RUnlock()

		if ok && timer.Stop() {
			s.mapMu.Lock()
			delete(s.repTimer, team+argSlot)
			s.mapMu.Unlock()

		}

		say := fmt.Sprintf("Reporting %s %s: %s", strings.ToUpper(team), strings.ToUpper(argSlot), name)
		s.rcon.Say(say)

	case 1:
		//first report happened, reset reps two minute later to 0, unless told to stop
		timer := time.AfterFunc(2*time.Minute, func() {
			ResetReportCount(target, s.LobbyId)
			say := fmt.Sprintf("Reporting %s %s failed, couldn't get enough votes in 2 minutes.", strings.ToUpper(team), strings.ToUpper(argSlot))
			s.rcon.Say(say)

		})

		s.mapMu.Lock()
		s.repTimer[team+argSlot] = timer
		s.mapMu.Unlock()
	}
	say := fmt.Sprintf("Got %d votes for reporting %s (%d needed)", curReps, name, repsNeeded[s.Type])
	s.rcon.Say(say)

	return
}
