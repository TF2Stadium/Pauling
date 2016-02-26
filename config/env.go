package config

import (
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/kelseyhightower/envconfig"
)

type constants struct {
	PortProfiler   string `envconfig:"PROFILER_PORT" default:"6061"`
	ProfilerEnable bool   `envconfig:"PROFILER_ENABLE" default:"false"`

	PrintLogMessages bool   `envconfig:"PRINT_LOG_MESSAGES" default:"false"`
	LogsPort         string `envconfig:"LOGS_PORT" default:"8002"`
	// PortMQ           string `envconfig:"MQ_PORT"`
	// AddrMQCtl        string `env:"MQ_CTL_ADDR"` // must include schema
	RPCQueue string `envconfig:"RPC_QUEUE" default:"pauling"`
	//HelenAddr string `envconfig:"HELEN_ADDR" default:"localhost:8081"`

	//EtcdAddr    string `envconfig:"ETCD_ADDR"`
	//EtcdService string `envconfig:"ETCD_SERVICE"`

	LogsTFAPIKey  string `envconfig:"LOGSTF_KEY"`
	RabbitMQURL   string `envconfig:"RABBITMQ_URL" default:"amqp://guest:guest@localhost:5672/"`
	RabbitMQQueue string `envconfig:"RABBITMQ_QUEUE" default:"events"`

	DBAddr     string `envconfig:"DATABASE_ADDR" default:"127.0.0.1:5432"`
	DBDatabase string `envconfig:"DATABASE_NAME" default:"tf2stadium"`
	DBUsername string `envconfig:"DATABASE_USERNAME" default:"tf2stadium"`
	DBPassword string `envconfig:"DATABASE_PASSWORD" default:"dickbutt"`
}

var Constants = constants{}

func InitConstants() {
	err := envconfig.Process("PAULING", &Constants)
	if err != nil {
		helpers.Logger.Fatal(err.Error())
	}
}
