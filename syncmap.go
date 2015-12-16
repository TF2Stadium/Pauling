package main

import (
	"errors"
	"sync"
)

var (
	servers = make(map[uint]*Server)
	mu      = new(sync.RWMutex)

	ErrNoServer = errors.New("Server doesn't exist.")
)

func getServer(id uint) (s *Server, err error) {
	var exists bool

	mu.RLock()
	s, exists = servers[id]
	mu.RUnlock()

	if !exists {
		err = ErrNoServer
	}

	return
}

func setServer(id uint, s *Server) {
	mu.Lock()
	servers[id] = s
	mu.Unlock()
}

func deleteServer(id uint) {
	mu.Lock()
	delete(servers, id)
	mu.Unlock()
}
