package rpc

import (
	"errors"
	"net"
	"net/rpc"
	"syscall"
	"time"

	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/TF2Stadium/Pauling/server"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
	rconwrapper "github.com/TF2Stadium/TF2RconWrapper"
	"github.com/james4k/rcon"
	"github.com/streadway/amqp"
	"github.com/vibhavp/amqp-rpc"
)

type Pauling struct{}
type Noreply struct{}

func StartRPC(url string) {
	conn, err := amqp.Dial(url)
	if err != nil {
		helpers.Logger.Fatal(err)
	}

	serverCodec, err := amqprpc.NewServerCodec(conn, config.Constants.RPCQueue, amqprpc.JSONCodec{})
	pauling := new(Pauling)
	rpc.Register(pauling)
	rpc.ServeCodec(serverCodec)
}

func (Pauling) VerifyInfo(info *models.ServerRecord, nop *Noreply) error {
	rc, err := rconwrapper.NewTF2RconConnection(info.Host, info.RconPassword)
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
	if rc == nil {
		return errors.New("Couldn't connect to server")
	}

	rc.Query("log on")
	if !server.Listener.TestSource(rc) {
		return errors.New("Log redirection not working")
	}

	return nil
}

func (Pauling) SetupServer(args *models.Args, nop *Noreply) error {
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

func (Pauling) ReExecConfig(args *models.Args, nop *Noreply) error {
	s, err := server.GetServer(args.Id)
	if err != nil {
		return err
	}

	s.Reset(args.ChangeMap)
	return nil
}

func (Pauling) End(args *models.Args, nop *Noreply) error {
	s, err := server.GetServer(args.Id)
	if err != nil {
		return err
	}

	server.DeleteServer(s.LobbyId)
	//now := time.Now().Unix()
	s.StopListening()
	//Logger.Debug("%d", time.Now().Unix()-now)

	return nil
}

func (Pauling) DisallowPlayer(args *models.Args, nop *Noreply) error {
	s, err := server.GetServer(args.Id)
	if err != nil {
		return err
	}

	steamID, _ := steamid.CommIdToSteamId(args.SteamId) //legacy steam id
	server.ResetReportCount(steamID, args.Id)

	err = s.KickPlayer(args.SteamId, "[tf2stadium.com] You have been replaced.")

	return nil
}

func (Pauling) Say(args *models.Args, nop *Noreply) error {
	s, err := server.GetServer(args.Id)
	if err != nil {
		return err
	}

	return s.Say(args.Text)
}

func (Pauling) Exists(lobbyID uint, reply *bool) error {
	_, err := server.GetServer(lobbyID)
	*reply = err == nil

	return nil
}
