package main

import (
	"sync"

	"github.com/TF2Stadium/Helen/models"
	"github.com/bitly/go-simplejson"
)

type Pauling int
type Noreply struct{}

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
	LobbyServerMap[args.Id].AllowedPlayers[args.CommId] = true
	return nil
}

func (_ *Pauling) DisallowPlayer(args *models.Args, nop *Noreply) error {
	s := LobbyServerMap[args.Id]
	if s.IsPlayerAllowed(args.CommId) {
		delete(s.AllowedPlayers, args.CommId)
	}
	return nil
}

func (_ *Pauling) GetEvent(args *models.Args, jsonStr *string) error {
	e := PopEvent()
	bytes, _ := e.Encode()
	*jsonStr = string(bytes)
	return nil
}
