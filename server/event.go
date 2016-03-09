package server

import (
	"encoding/json"

	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/streadway/amqp"
)

var (
	conn    *amqp.Connection
	queue   amqp.Queue
	channel *amqp.Channel
)

//Mirrored across github.com/TF2Stadium/Helen/event
type Event struct {
	Name    string
	SteamID string

	LobbyID    uint
	LogsID     int //logs.tf ID
	ClassTimes map[string]*classTime
}

const (
	PlayerDisconnected string = "playerDisc"
	PlayerSubstituted  string = "playerSub"
	PlayerConnected    string = "playerConn"
	PlayerChat         string = "playerChat"

	DisconnectedFromServer string = "discFromServer"
	MatchEnded             string = "matchEnded"
	Test                   string = "test"
)

func connectMQ() {
	var err error

	conn, err = amqp.Dial(config.Constants.RabbitMQURL)
	if err != nil {
		helpers.Logger.Fatalf("Failed to connect to RabbitMQ - %s", err.Error())
	}

	channel, err = conn.Channel()
	if err != nil {
		helpers.Logger.Fatalf("Failed to open a channel - %s", err.Error())
	}

	queue, err = channel.QueueDeclare(
		config.Constants.RabbitMQQueue, // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)

	if err != nil {
		helpers.Logger.Fatalf("Failed to declare a queue - %s", err.Error())
	}

	helpers.Logger.Info("Sending events on queue %s on %s", config.Constants.RabbitMQQueue, config.Constants.RabbitMQURL)
}

func publishEvent(e Event) {
	bytes, _ := json.Marshal(e)
	channel.Publish(
		"",
		queue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        bytes,
		})
}
