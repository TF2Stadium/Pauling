package config

import (
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/kelseyhightower/envconfig"
)

type constants struct {
	PortProfiler   string `envconfig:"PROFILER_PORT" default:"6061"`
	ProfilerEnable bool   `envconfig:"PROFILER_ENABLE" default:"false"`

	PrintLogMessages bool   `envconfig:"PRINT_LOG_MESSAGES" default:"false"`
	PortRcon         string `envconfig:"RCON_PORT" default:"8002"`
	PortRPC          string `envconfig:"RPC_PORT" default:"8001"`
	PortHelen        string `envconfig:"HELEN_PORT" default:"8081"`
	PortMQ           string `envconfig:"LOGSTF_KEY"`
	LogsTFAPIKey     string `envconfig:"MQ_PORT"`
}

var Constants = constants{}

func InitConstants() {
	err := envconfig.Process("PAULING", &Constants)
	if err != nil {
		helpers.Logger.Fatal(err.Error())
	}
}
