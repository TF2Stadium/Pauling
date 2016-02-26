package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TF2Stadium/Helen/models"
	"github.com/stretchr/testify/assert"
)

func TestConfigName(t *testing.T) {
	t.Parallel()
	configPath, _ := filepath.Abs("../configs/")
	cases := []struct {
		mapName   string
		lobbyType models.LobbyType
		ruleset   string

		config string
	}{
		{"cp_badlands", models.LobbyTypeSixes, "ugc", "ugc/cp_sixes.cfg"},
		{"cp_process_final", models.LobbyTypeHighlander, "ugc", "ugc/cp_highlander.cfg"},

		{"pl_badwater", models.LobbyTypeHighlander, "etf2l", "etf2l/pl_highlander.cfg"},

		{"ctf_turbine", models.LobbyTypeSixes, "ugc", "ugc/ctf_sixes.cfg"},

		{"koth_lakeside", models.LobbyTypeHighlander, "ugc", "ugc/koth_highlander.cfg"},
		{"koth_viaduct", models.LobbyTypeSixes, "ugc", "ugc/koth_sixes.cfg"},

		{"ctf_ballin", models.LobbyTypeBball, "etf2l", "etf2l/ctf_bball.cfg"},
		{"ultiduo_balloo", models.LobbyTypeUltiduo, "etf2l", "etf2l/koth_ultiduo.cfg"},
	}

	for _, test := range cases {
		name, err := ConfigName(test.mapName, test.lobbyType, test.ruleset)
		assert.NoError(t, err, "map %s | lobby type %s", test.mapName, formatMap[test.lobbyType])
		assert.Equal(t, name, test.config)
		file, err := os.Open(configPath + "/" + test.config)
		assert.NoError(t, err)
		file.Close()
	}
}
