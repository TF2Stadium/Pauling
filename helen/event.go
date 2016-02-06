package helen

import "github.com/TF2Stadium/Helen/rpc"

func SendEvent(e rpc.Event) { helenClient.Call("Event.Handle", e, struct{}{}) }
