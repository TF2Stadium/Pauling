package helen

import (
	"github.com/TF2Stadium/Helen/models"
)

func GetServers() map[uint]*models.ServerRecord {
	servers := make(map[uint]*models.ServerRecord)
	Call("Helen.GetServers", struct{}{}, &servers)
	return servers
}