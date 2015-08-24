package main

import (
	"container/list"
	"sync"
)

var EventQueue = &list.List{}
var EventQueueMutex = &sync.Mutex{}

type Event map[string]interface{}

const (
	EventTest                  = "test"
	EventPlayerDiscconected    = "playerDisc"
	EventPlayerConnected       = "playerConn"
	EventDisconectedFromServer = "discFromServer"
	EventMatchEnded            = "matchEnded"
)

func (e *Event) CopyFrom(e2 Event) {
	for key, value := range e2 {
		(*e)[key] = value
	}
}

func PushEvent(name string, value ...interface{}) {
	event := make(Event)
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

func PopEvent() Event {
	EventQueueMutex.Lock()
	val := EventQueue.Front()
	EventQueueMutex.Unlock()

	if val == nil {
		return nil
	}
	return val.Value.(Event)
}
