package server

import (
	"github.com/TF2Stadium/Helen/rpc"
	"github.com/TF2Stadium/Pauling/helen"
)

func send(e rpc.Event) { helen.Call("Event.Handle", e, struct{}{}) }

func PlayerDisconnected(lobbyID, playerID uint) {
	e := rpc.Event{
		Name:     rpc.PlayerDisconnected,
		LobbyID:  lobbyID,
		PlayerID: playerID,
	}

	send(e)
}

func PlayerConnected(lobbyID, playerID uint) {
	e := rpc.Event{
		Name:     rpc.PlayerConnected,
		LobbyID:  lobbyID,
		PlayerID: playerID}
	send(e)
}

func DisconnectedFromServer(lobbyID uint) {
	e := rpc.Event{
		Name:    rpc.DisconnectedFromServer,
		LobbyID: lobbyID,
	}
	send(e)
}

func Substitute(lobbyID, playerID uint) {
	e := rpc.Event{
		Name:     rpc.PlayerSubstituted,
		LobbyID:  lobbyID,
		PlayerID: playerID,
	}
	send(e)
}

func MatchEnded(lobbyID uint, logsID int) {
	e := rpc.Event{
		Name:    rpc.MatchEnded,
		LobbyID: lobbyID,
		LogsID:  logsID,
	}
	send(e)
}
