package helen

import (
	"net/rpc"

	"github.com/TF2Stadium/Pauling/internal/helpers"
)

var Client *rpc.Client

func Connect(port string) {
	var err error
	helpers.Logger.Info("Connecting to Helen RPC on localhost:%s", port)
	Client, err = rpc.DialHTTP("tcp", "localhost:"+port)
	if err != nil {
		helpers.Logger.Fatal(err)
	}
	helpers.Logger.Info("Connected!")
}
