package main

import (
	"container/list"
	"sync"

	"github.com/bitly/go-simplejson"
)

var EventQueue = &list.List{}
var EventQueueMutex = &sync.Mutex{}

type Event string

const (
	EventPlayerDiscconected    Event = "playerDisc"
	EventPlayerConnected       Event = "playerConn"
	EventDisconectedFromServer Event = "discFromServer"
	EventMatchEnded            Event = "matchEnded"
)

func PushEvent(event Event, value ...interface{}) {
	json := simplejson.New()
	json.Set("event", event)

	switch event {
	case EventPlayerDiscconected, EventPlayerConnected:
		json.Set("lobbyId", value[0].(string))
		json.Set("commId", value[1].(string))
	case EventDisconectedFromServer, EventMatchEnded:
		json.Set("lobbyId", value[0].(uint))
	}

	EventQueueMutex.Lock()
	EventQueue.PushBack(json)
	EventQueueMutex.Unlock()
}

func PopEvent() *simplejson.Json {
	EventQueueMutex.Lock()
	val := EventQueue.Front()
	EventQueueMutex.Unlock()

	if val == nil {
		return simplejson.New()
	}
	return val.Value.(*simplejson.Json)
}
