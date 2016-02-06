package helen

import (
	"sync"

	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/vibhavp/rpcconn"
)

var (
	helenClient *rpcconn.Client
	mu          = new(sync.RWMutex)
)

func Connect(addr string) {
	var err error
	mu.Lock()
	defer mu.Unlock()

	helpers.Logger.Info("Connecting to Helen on %s", addr)
	helenClient, err = rpcconn.DialHTTP("tcp", addr)
	for err != nil {
		helpers.Logger.Fatal(err)
	}
}
