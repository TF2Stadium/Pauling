package main

import (
	"github.com/TF2Stadium/Helen/models"
)

var EventQueue = make(chan models.Event, 100)

const (
	EventTest                  = "test"
	EventPlayerDiscconected    = "playerDisc"
	EventPlayerConnected       = "playerConn"
	EventDisconectedFromServer = "discFromServer"
	EventMatchEnded            = "matchEnded"
	EventSubstitute            = "playerSub"
)

func PushEvent(name string, value ...interface{}) {
	event := make(models.Event)
	event["name"] = name

	switch name {
	case EventPlayerDiscconected, EventPlayerConnected:
		event["lobbyId"] = value[0].(uint)
		event["steamId"] = value[1].(string)
	case EventSubstitute:
		event["lobbyId"] = value[0].(uint)
		event["steamId"] = value[1].(string)
	case EventDisconectedFromServer, EventMatchEnded:
		event["lobbyId"] = value[0].(uint)
	}

	EventQueue <- event
}
