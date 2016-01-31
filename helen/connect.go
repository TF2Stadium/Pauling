package helen

import (
	"io"
	"net"
	"net/rpc"
	"sync"
	"time"

	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/helpers"
)

var (
	client *rpc.Client
	mu     = new(sync.RWMutex)
)

func Connect(port string) {
	var err error
	mu.Lock()
	defer mu.Unlock()

	client, err = rpc.DialHTTP("tcp", "localhost:"+port)
	for err != nil {
		helpers.Logger.Error(err.Error())
		time.Sleep(1 * time.Second)
		client, err = rpc.DialHTTP("tcp", "localhost:"+port)
	}
}

func isNetworkError(err error) bool {
	_, ok := err.(*net.OpError)
	return ok || err == io.ErrUnexpectedEOF || err == rpc.ErrShutdown

}

func Call(method string, args interface{}, reply interface{}) error {
	mu.RLock()
	err := client.Call(method, args, reply)
	mu.RUnlock()

	if isNetworkError(err) {
		Connect(config.Constants.PortHelen)
		//retry call again
		return Call(method, args, reply)
	}

	return err
}
