package rpc

import (
	"errors"
	"net"
	"net/http"
	"net/rpc"
	"net/url"
	"sync"
	"syscall"
	"time"

	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/helen"
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/TF2Stadium/Pauling/server"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
	rconwrapper "github.com/TF2Stadium/TF2RconWrapper"
	"github.com/james4k/rcon"
)

type Pauling struct{}
type Noreply struct{}

func StartRPC(l net.Listener) {
	pauling := new(Pauling)
	rpc.Register(pauling)
	rpc.HandleHTTP()

	helpers.Logger.Info("Listening on %s", config.Constants.AddrRPC)
	helpers.Logger.Fatal(http.Serve(l, nil))
}

func (_ *Pauling) VerifyInfo(info *models.ServerRecord, nop *Noreply) error {
	c, err := rconwrapper.NewTF2RconConnection(info.Host, info.RconPassword)
	if c != nil {
		defer c.Close()

		c.Query("log on; sv_rcon_log 1; sv_logflush 1")
		listener := server.RconListener.CreateServerListener(c)

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

func (_ *Pauling) SetupServer(args *models.Args, nop *Noreply) error {
	s := server.NewServer()
	s.LobbyId = args.Id
	s.Info = args.Info
	s.Type = args.Type
	s.League = args.League
	s.Whitelist = args.Whitelist
	s.Map = args.Map

	err := s.Setup()
	if err != nil {
		//Logger.Warning(err.Error())
		return err
	}

	go s.StartVerifier(time.NewTicker(time.Second * 10))

	server.SetServer(args.Id, s)
	return nil
}

func (_ *Pauling) ReExecConfig(args *models.Args, nop *Noreply) error {
	s, err := server.GetServer(args.Id)
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
	s, err := server.GetServer(args.Id)
	if err != nil {
		return err
	}

	server.DeleteServer(s.LobbyId)
	//now := time.Now().Unix()
	go s.ServerListener.Close(s.Rcon)
	s.StopLogListener <- struct{}{}
	//Logger.Debug("%d", time.Now().Unix()-now)

	return nil
}

func (_ *Pauling) DisallowPlayer(args *models.Args, nop *Noreply) error {
	s, err := server.GetServer(args.Id)
	if err != nil {
		return err
	}

	id, _ := steamid.CommIdToSteamId(args.SteamId)
	server.ResetReportCount(id, args.Id)

	players, _ := s.Rcon.GetPlayers()
	for _, player := range players {
		if player.SteamID == id {
			s.Rcon.KickPlayer(player, "[tf2stadium.com] You have been replaced.")
		}
	}

	return nil
}

func (*Pauling) Say(args *models.Args, nop *Noreply) error {
	s, err := server.GetServer(args.Id)
	if err != nil {
		return err
	}

	return s.Rcon.Say(args.Text)
}

func (Pauling) Exists(lobbyID uint, reply *bool) error {
	_, err := server.GetServer(lobbyID)
	*reply = err == nil

	return nil
}

var once = new(sync.Once)

func (Pauling) Ping(struct{}, *struct{}) error {
	once.Do(func() {
		helen.Connect(config.Constants.HelenAddr)
		server.SetupServers()

		if config.Constants.AddrMQCtl != "" {
			u, err := url.Parse(config.Constants.AddrMQCtl)
			if err != nil && config.Constants.AddrMQCtl != "" {
				helpers.Logger.Fatal(err)
			}

			u.Path = "start"
			_, err = http.Post(u.String(), "", nil)
			if err != nil && config.Constants.AddrMQCtl != "" {
				helpers.Logger.Fatal(err)
			}
		}

	})
	return nil
}
