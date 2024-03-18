package processors

import (
	"github.com/streadway/amqp"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gostreambridge/pkg/util"
)

func ConsumeAMQPMessages() <-chan []byte {
	messages := make(chan []byte)

	// Reading configuration
	amqpConfig := util.ConvertConfigFileToMap("rabbitmq.json")

	// Connect to RabbitMQ server
	conn, err := amqp.Dial(amqpConfig["url"])
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	// Create a channel
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	// Declare a queue
	q, err := ch.QueueDeclare(
		amqpConfig["queue"], // name
		false,               // durable
		false,               // delete when unused
		false,               // exclusive
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare a queue: %v", err)
	}

	// Consume messages from the queue
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	// Channel to handle OS signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming messages
	go func() {
		for msg := range msgs {
			log.Printf("Received message: %s\n", msg.Body)
			messages <- msg.Body
		}
	}()

	return messages
}
