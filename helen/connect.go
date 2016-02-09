package helen

import (
	"sync"

	helenHelpers "github.com/TF2Stadium/Helen/helpers"
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
	helenClient, err = rpcconn.DialHTTP("tcp", helenHelpers.Address{addr})
	for err != nil {
		helpers.Logger.Fatal(err)
	}
}
