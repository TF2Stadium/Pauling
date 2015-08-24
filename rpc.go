package main

import (
	"errors"
	"sync"

	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/PlayerStatsScraper/steamid"
)

type Pauling int
type Noreply struct{}

func (_ *Pauling) SetupVerifier(args *models.ServerBootstrap, nop *Noreply) error {
	s := NewServer()
	LobbyMutexMap[args.LobbyId].Lock()
	LobbyServerMap[args.LobbyId] = s
	LobbyMutexMap[args.LobbyId].Unlock()
	s.LobbyId = args.LobbyId
	s.Info = args.Info
	for _, playerId := range args.BannedPlayers {
		s.AllowedPlayers[playerId] = false
	}
	for _, playerId := range args.Players {
		s.AllowedPlayers[playerId] = true
	}
	s.StartVerifier()

	return nil
}

func (_ *Pauling) SetupServer(args *models.Args, nop *Noreply) error {
	s := NewServer()
	s.LobbyId = args.Id
	s.Info = args.Info
	s.Type = args.Type
	s.League = args.League
	s.Map = args.Map

	err := s.Setup()
	if err != nil {
		Logger.Warning(err.Error())
		return err
	}

	err = s.StartVerifier()
	if err != nil {
		Logger.Warning(err.Error())
		return err
	}

	LobbyServerMap[s.LobbyId] = s
	LobbyMutexMap[s.LobbyId] = &sync.Mutex{}
	return nil
}

func (_ *Pauling) End(args *models.Args, nop *Noreply) error {
	s := LobbyServerMap[args.Id]
	s.End()
	return nil
}

func (_ *Pauling) AllowPlayer(args *models.Args, nop *Noreply) error {
	commId, _ := steamid.SteamIdToCommId(args.SteamId)
	LobbyServerMap[args.Id].AllowedPlayers[commId] = true
	return nil
}

func (_ *Pauling) DisallowPlayer(args *models.Args, nop *Noreply) error {
	s := LobbyServerMap[args.Id]
	commId, _ := steamid.SteamIdToCommId(args.SteamId)
	if s.IsPlayerAllowed(commId) {
		delete(s.AllowedPlayers, commId)
	}
	return nil
}

func (_ *Pauling) GetEvent(args *models.Args, event *Event) error {
	e := PopEvent()
	if e == nil {
		return errors.New("Event queue empty")
	}
	event.CopyFrom(e)
	return nil
}
