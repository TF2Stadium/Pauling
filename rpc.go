package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"syscall"
	"time"

	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
	rconwrapper "github.com/TF2Stadium/TF2RconWrapper"
	"github.com/james4k/rcon"
)

type Pauling struct{}
type Noreply struct{}

func startRPC() {
	pauling := new(Pauling)
	rpc.Register(pauling)
	rpc.HandleHTTP()

	port := "8001"
	overrideFromEnv(&port, "PAULING_PORT")

	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%s", port))
	if err != nil {
		Logger.Fatal(err)
	}
	Logger.Info("Listening on %s", port)
	Logger.Fatal(http.Serve(l, nil))
}

func (_ *Pauling) VerifyInfo(info *models.ServerRecord, nop *Noreply) error {
	c, err := rconwrapper.NewTF2RconConnection(info.Host, info.RconPassword)
	if c != nil {
		defer c.Close()

		c.Query("log on; sv_rcon_log 1; sv_logflush 1")
		listener := RconListener.CreateServerListener(c)

		tick := time.After(time.Second * 5)
		err := make(chan error)

		go func() {
			select {
			case <-tick:
				listener.Close(c)
				err <- errors.New("Server doesn't support log redirection. Make sure your server isn't blocking outgoing logs.")
			case <-listener.RawMessages:
				listener.Close(c)
				err <- nil
			}
		}()
		c.Query("sv_password")

		return <-err
	}
	if err != nil {
		switch err.(type) {
		case *net.OpError:
			if err.(*net.OpError).Timeout() {
				return errors.New("Couldn't connect to the server: Connection timed out.")
			}
		case syscall.Errno:
			if err.(syscall.Errno) == syscall.ECONNREFUSED {
				return errors.New("Couldn't connect to the server: Connection Refused.")
			}
			return err

		default:
			if err == rcon.ErrAuthFailed {
				return errors.New("Authentication Failed. Please check your RCON Address/Password.")
			}

		}
	}
	return errors.New("Couldn't connect to the server.")
}

func (_ *Pauling) SetupVerifier(args *models.ServerBootstrap, nop *Noreply) error {
	s := NewServer()
	setServer(args.LobbyId, s)

	s.LobbyId = args.LobbyId
	s.Info = args.Info

	s.AllowedPlayers.Lock()
	defer s.AllowedPlayers.Unlock()

	for _, playerId := range args.BannedPlayers {
		s.AllowedPlayers.Map[playerId] = false
	}
	for _, playerId := range args.Players {
		s.AllowedPlayers.Map[playerId] = true
	}
	go s.StartVerifier(time.NewTicker(time.Second * 10))

	return nil
}

func (_ *Pauling) SetupServer(args *models.Args, nop *Noreply) error {
	s := NewServer()
	s.LobbyId = args.Id
	s.Info = args.Info
	s.Type = args.Type
	s.League = args.League
	s.Whitelist = args.Whitelist
	s.Map = args.Map

	err := s.Setup()
	if err != nil {
		//Logger.Warning(err.Error())
		Logger.Debug("#%d: Error while setting up: %s", args.Id, err.Error())
		return err
	}

	go s.StartVerifier(time.NewTicker(time.Second * 10))

	setServer(args.Id, s)
	return nil
}

func (_ *Pauling) ReExecConfig(args *models.Args, nop *Noreply) error {
	s, err := getServer(args.Id)
	if err != nil {
		return err
	}

	err = s.ExecConfig()
	if err != nil {
		return err
	}

	err = s.Rcon.ChangeMap(s.Map)

	return err
}

func (_ *Pauling) End(args *models.Args, nop *Noreply) error {
	s, err := getServer(args.Id)
	if err != nil {
		return err
	}

	deleteServer(s.LobbyId)
	//now := time.Now().Unix()
	s.StopLogListener <- struct{}{}
	//Logger.Debug("%d", time.Now().Unix()-now)

	return nil
}

func (_ *Pauling) AllowPlayer(args *models.Args, nop *Noreply) error {
	s, err := getServer(args.Id)
	if err != nil {
		return err
	}

	s.AllowedPlayers.Lock()
	s.AllowedPlayers.Map[args.SteamId] = true
	s.AllowedPlayers.Unlock()

	s.Slots.Lock()
	id, _ := steamid.CommIdToSteamId(args.SteamId)
	s.Slots.Map[args.Slot] = id
	s.Slots.Unlock()

	return nil
}

func (_ *Pauling) DisallowPlayer(args *models.Args, nop *Noreply) error {
	s, err := getServer(args.Id)
	if err != nil {
		return err
	}

	if !s.IsPlayerAllowed(args.SteamId) {
		return nil
	}

	s.AllowedPlayers.Lock()
	defer s.AllowedPlayers.Unlock()
	delete(s.AllowedPlayers.Map, args.SteamId)

	s.Slots.Lock()
	id, _ := steamid.CommIdToSteamId(args.SteamId)
	for slot, steamID := range s.Slots.Map {
		if steamID == id {
			delete(s.Slots.Map, slot)
		}
	}
	s.Slots.Unlock()

	players, _ := s.Rcon.GetPlayers()
	for _, player := range players {
		if player.SteamID == id {
			s.Rcon.KickPlayer(player, "[tf2stadium.com] You have been replaced.")
		}
	}

	return nil
}

func (_ *Pauling) GetEvent(args *models.Args, event *models.Event) error {
	*event = <-EventQueue
	return nil
}

func (*Pauling) Say(args *models.Args, nop *Noreply) error {
	s, err := getServer(args.Id)
	if err != nil {
		return err
	}

	return s.Rcon.Say(args.Text)
}
