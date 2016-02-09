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
	PortMQ           string `envconfig:"MQ_PORT"`
	AddrMQCtl        string `env:"MQ_CTL_ADDR"` // must include schema
	AddrRPC          string `envconfig:"RPC_ADDR" default:"localhost:8001"`
	LogsTFAPIKey     string `envconfig:"LOGSTF_KEY"`

	HelenAddr string `envconfig:"HELEN_ADDR" default:"localhost:8081"`

	EtcdAddr    string `envconfig:"ETCD_ADDR"`
	EtcdService string `envconfig:"ETCD_SERVICE"`
}

var Constants = constants{}

func InitConstants() {
	err := envconfig.Process("PAULING", &Constants)
	if err != nil {
		helpers.Logger.Fatal(err.Error())
	}
}
