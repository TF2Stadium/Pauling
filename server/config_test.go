package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TF2Stadium/Helen/models/lobby/format"
	"github.com/stretchr/testify/assert"
)

func TestConfigName(t *testing.T) {
	t.Parallel()
	configPath, _ := filepath.Abs("../configs/")
	cases := []struct {
		mapName   string
		lobbyType format.Format
		ruleset   string

		config string
	}{
		{"cp_badlands", format.Sixes, "ugc", "ugc/cp_sixes.cfg"},
		{"cp_process_final", format.Highlander, "ugc", "ugc/cp_highlander.cfg"},

		{"pl_badwater", format.Highlander, "etf2l", "etf2l/pl_highlander.cfg"},

		{"ctf_turbine", format.Sixes, "ugc", "ugc/ctf_sixes.cfg"},

		{"koth_lakeside", format.Highlander, "ugc", "ugc/koth_highlander.cfg"},
		{"koth_viaduct", format.Sixes, "ugc", "ugc/koth_sixes.cfg"},

		{"ctf_ballin", format.Bball, "etf2l", "etf2l/ctf_bball.cfg"},
		{"ultiduo_balloo", format.Ultiduo, "etf2l", "etf2l/koth_ultiduo.cfg"},

		{"cp_gravelpit", format.Highlander, "etf2l", "etf2l/cp_highlander_stopwatch.cfg"},
		{"cp_steel", format.Highlander, "ugc", "ugc/cp_highlander_stopwatch.cfg"},
		{"cp_gravelpit", format.Sixes, "etf2l", "etf2l/cp_sixes_stopwatch.cfg"},
	}

	for _, test := range cases {
		name, err := ConfigName(test.mapName, test.lobbyType, test.ruleset)
		assert.NoError(t, err, "map %s | lobby type %s", test.mapName, formatMap[test.lobbyType])
		assert.Equal(t, name, test.config)
		_, err = os.Open(configPath + "/" + test.config)
		assert.NoError(t, err)
	}
}
