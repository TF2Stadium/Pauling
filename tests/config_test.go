package tests

import (
	"testing"

	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/Pauling"
	"github.com/stretchr/testify/assert"
)

func TestConfigFileName(t *testing.T) {
	main.InitConfigs()
	name, _ := main.ConfigFileName("cp_badlands", models.LobbyTypeSixes, main.LeagueEtf2l)
	assert.Equal(t, name, main.ConfigsPath+"/etf2l/etf2l_6v6_5cp.cfg")
}
