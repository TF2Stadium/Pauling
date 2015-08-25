package main

import (
	"container/list"
	"sync"

	"github.com/TF2Stadium/Helen/models"
)

var EventQueue = &list.List{}
var EventQueueMutex = &sync.Mutex{}

const (
	EventTest                  = "test"
	EventPlayerDiscconected    = "playerDisc"
	EventPlayerConnected       = "playerConn"
	EventDisconectedFromServer = "discFromServer"
	EventMatchEnded            = "matchEnded"
)

func PushEvent(name string, value ...interface{}) {
	event := make(models.Event)
	event["name"] = name

	switch name {
	case EventPlayerDiscconected, EventPlayerConnected:
		event["lobbyId"] = value[0].(string)
		event["commId"] = value[1].(string)
	case EventDisconectedFromServer, EventMatchEnded:
		event["lobbyId"] = value[0].(uint)
	}

	EventQueueMutex.Lock()
	EventQueue.PushBack(event)
	EventQueueMutex.Unlock()
}

func PopEvent() models.Event {
	EventQueueMutex.Lock()
	val := EventQueue.Front()
	if val != nil {
		EventQueue.Remove(val)
	}
	EventQueueMutex.Unlock()

	if val == nil {
		return nil
	}
	return val.Value.(models.Event)
}
