package event

import (
	"github.com/TF2Stadium/Helen/models"
)

var eventQueue = make(chan models.Event, 100)

const (
	Test                  = "test"
	PlayerDisconnected    = "playerDisc"
	PlayerConnected       = "playerConn"
	DisconectedFromServer = "discFromServer"
	MatchEnded            = "matchEnded"
	Substitute            = "playerSub"
)

func Get() models.Event {
	return <-eventQueue
}

func Push(name string, value ...interface{}) {
	event := make(models.Event)
	event["name"] = name

	switch name {
	case PlayerDisconnected, PlayerConnected:
		event["lobbyId"] = value[0].(uint)
		event["steamId"] = value[1].(string)
	case Substitute:
		event["lobbyId"] = value[0].(uint)
		event["steamId"] = value[1].(string)
	case DisconectedFromServer, MatchEnded:
		event["lobbyId"] = value[0].(uint)
	}

	eventQueue <- event
}
