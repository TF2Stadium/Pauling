package helen

import (
	"github.com/TF2Stadium/Helen/models"
)

var servers = make(map[uint]*models.ServerRecord)

func GetServers() map[uint]*models.ServerRecord {
	return servers
}
