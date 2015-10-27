package main

import (
	"github.com/TF2Stadium/Helen/models"
	rconwrapper "github.com/TF2Stadium/TF2RconWrapper"
)

type Pauling int
type Noreply struct{}

func (_ *Pauling) VerifyInfo(info *models.ServerRecord, nop *Noreply) error {
	_, err := rconwrapper.NewTF2RconConnection(info.Host, info.RconPassword)

	return err
}

func (_ *Pauling) SetupVerifier(args *models.ServerBootstrap, nop *Noreply) error {
	s := NewServer()
	ServerMap.Lock()
	ServerMap.Map[args.LobbyId] = s
	ServerMap.Unlock()

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
		Logger.Warning(err.Error())
		return err
	}

	NewServerChan <- s

	ServerMap.Lock()
	ServerMap.Map[s.LobbyId] = s
	ServerMap.Unlock()
	return nil
}

func (_ *Pauling) End(args *models.Args, nop *Noreply) error {
	ServerMap.RLock()
	s := ServerMap.Map[args.Id]
	ServerMap.RUnlock()

	s.StopVerifier <- true
	return nil
}

func (_ *Pauling) AllowPlayer(args *models.Args, nop *Noreply) error {
	ServerMap.RLock()
	s := ServerMap.Map[args.Id]
	ServerMap.RUnlock()

	s.AllowedPlayers.Lock()
	s.AllowedPlayers.Map[args.SteamId] = true
	s.AllowedPlayers.Unlock()

	return nil
}

func (_ *Pauling) DisallowPlayer(args *models.Args, nop *Noreply) error {
	ServerMap.RLock()
	s := ServerMap.Map[args.Id]
	ServerMap.RUnlock()

	if s.IsPlayerAllowed(args.SteamId) {
		s.AllowedPlayers.Lock()
		defer s.AllowedPlayers.Unlock()
		delete(s.AllowedPlayers.Map, args.SteamId)
	}
	return nil
}

func (_ *Pauling) SubstitutePlayer(args *models.Args, nop *Noreply) error {
	ServerMap.RLock()
	s := ServerMap.Map[args.Id]
	ServerMap.RUnlock()

	s.Substitutes.Lock()
	s.Substitutes.Map[args.SteamId] = args.SteamId2
	s.Substitutes.Unlock()

	return nil
}

func (_ *Pauling) GetEvent(args *models.Args, event *models.Event) error {
	e := PopEvent()
	*event = e
	return nil
}
