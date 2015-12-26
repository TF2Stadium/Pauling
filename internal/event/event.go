package event

import (
	"github.com/TF2Stadium/Helen/models"
)

var eventQueue = make(chan models.Event, 100)

func Get() models.Event {
	return <-eventQueue
}

func PlayerDisconnected(lobbyID, playerID uint) {
	event := models.Event{
		"name":     "playerDisc",
		"lobbyID":  lobbyID,
		"playerID": playerID,
	}

	eventQueue <- event
}

func PlayerConnected(lobbyID, playerID uint) {
	event := models.Event{
		"name":     "playerConn",
		"lobbyID":  lobbyID,
		"playerID": playerID,
	}
	eventQueue <- event
}

func DisconnectedFromServer(lobbyID uint) {
	event := models.Event{
		"name":    "discFromServer",
		"lobbyID": lobbyID,
	}
	eventQueue <- event
}

func Substitute(lobbyID, playerID uint) {
	event := models.Event{
		"name":     "playerSub",
		"lobbyID":  lobbyID,
		"playerID": playerID,
	}

	eventQueue <- event
}

func MatchEnded(lobbyID uint) {
	event := models.Event{
		"name":    "matchEnded",
		"lobbyID": lobbyID,
	}
	eventQueue <- event
}
