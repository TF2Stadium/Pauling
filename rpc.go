package main

import (
	"errors"
	"sync"

	"github.com/TF2Stadium/Helen/models"
	rconwrapper "github.com/TF2Stadium/TF2RconWrapper"
)

type Pauling int
type Noreply struct{}

var serverMap = struct {
	Map map[uint]*Server
	*sync.RWMutex
}{make(map[uint]*Server), new(sync.RWMutex)}

var ErrNoServer = errors.New("Server doesn't exist.")

func getServer(id uint) (s *Server, err error) {
	var exists bool

	serverMap.RLock()
	s, exists = serverMap.Map[id]
	serverMap.RUnlock()

	if !exists {
		err = ErrNoServer
	}

	return
}

func setServer(id uint, s *Server) {
	serverMap.Lock()
	serverMap.Map[id] = s
	serverMap.Unlock()
}

func (_ *Pauling) VerifyInfo(info *models.ServerRecord, nop *Noreply) error {
	c, err := rconwrapper.NewTF2RconConnection(info.Host, info.RconPassword)
	if c != nil {
		_, err = c.Query("tftrue_gamedesc")
		if err != nil {
			err = errors.New("TFTrue isn't installed on the server.")
		}
		c.Close()
	}
	return err
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
	NewServerChan <- s

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
		return err
	}

	NewServerChan <- s

	setServer(args.Id, s)
	return nil
}

func (_ *Pauling) ReExecConfig(args *models.Args, nop *Noreply) error {
	serverMap.RLock()
	s, ok := serverMap.Map[args.Id]
	serverMap.RUnlock()

	if !ok {
		return ErrNoServer
	}

	err := s.ExecConfig()
	if err != nil {
		return err
	}

	err = s.Rcon.ChangeMap(s.Map)

	return err
}

func (_ *Pauling) End(args *models.Args, nop *Noreply) error {
	s, err := getServer(args.Id)
	if err != nil {
		return ErrNoServer
	}

	//now := time.Now().Unix()
	s.StopVerifier <- struct{}{}
	//Logger.Debug("%d", time.Now().Unix()-now)

	return nil
}

func (_ *Pauling) AllowPlayer(args *models.Args, nop *Noreply) error {
	s, err := getServer(args.Id)
	if err != nil {
		return ErrNoServer
	}

	s.AllowedPlayers.Lock()
	s.AllowedPlayers.Map[args.SteamId] = true
	s.AllowedPlayers.Unlock()

	s.Slots.Lock()
	s.Slots.Map[args.Slot] = args.SteamId
	s.Slots.Unlock()

	return nil
}

func (_ *Pauling) DisallowPlayer(args *models.Args, nop *Noreply) error {
	s, err := getServer(args.Id)
	if err != nil {
		return ErrNoServer
	}

	if !s.IsPlayerAllowed(args.SteamId) {
		return nil
	}

	s.AllowedPlayers.Lock()
	defer s.AllowedPlayers.Unlock()
	delete(s.AllowedPlayers.Map, args.SteamId)

	s.Slots.Lock()
	for slot, steamID := range s.Slots.Map {
		if steamID == args.SteamId {
			delete(s.Slots.Map, slot)
		}
	}
	s.Slots.Unlock()

	return nil
}

func (_ *Pauling) GetEvent(args *models.Args, event *models.Event) error {
	*event = <-EventQueue
	return nil
}
