package config

import (
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/kelseyhightower/envconfig"
)

type constants struct {
	PrintLogMessages bool   `envconfig:"PRINT_LOG_MESSAGES" default:"false"`
	LogsPort         string `envconfig:"LOGS_PORT" default:"8002"`
	RPCQueue         string `envconfig:"RPC_QUEUE" default:"pauling"`

	LogsTFAPIKey  string `envconfig:"LOGSTF_KEY"`
	RabbitMQURL   string `envconfig:"RABBITMQ_URL" default:"amqp://guest:guest@localhost:5672/"`
	RabbitMQQueue string `envconfig:"RABBITMQ_QUEUE" default:"events"`

	DBAddr     string `envconfig:"DATABASE_ADDR" default:"127.0.0.1:5432"`
	DBDatabase string `envconfig:"DATABASE_NAME" default:"tf2stadium"`
	DBUsername string `envconfig:"DATABASE_USERNAME" default:"tf2stadium"`
	DBPassword string `envconfig:"DATABASE_PASSWORD" default:"dickbutt"`

	ProfilerAddr string `envconfig:"PROFILER_ADDR"`
}

var Constants = constants{}

func InitConstants() {
	err := envconfig.Process("PAULING", &Constants)
	if err != nil {
		helpers.Logger.Fatal(err.Error())
	}
}
