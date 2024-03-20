package processors

import (
	"github.com/streadway/amqp"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gostreambridge/pkg/util"
)

func ConsumeAMQPMessages() <-chan []byte {
	messages := make(chan []byte)

	// Reading configuration
	amqpConfig := util.ConvertConfigFileToMap("rabbitmq.json")

	var conn *amqp.Connection
	var err error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		conn, err = amqp.Dial(amqpConfig["url"])
		if err == nil {
			break
		}
		log.Printf("Failed to connect to RabbitMQ (attempt %d/%d): %v\n", i+1, maxRetries, err)
		time.Sleep(2 * time.Second) // Wait before retrying
	}
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ after %d retries: %v", maxRetries, err)
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

	// Retry consuming messages from the queue
	var msgs <-chan amqp.Delivery
	for i := 0; i < maxRetries; i++ {
		msgs, err = ch.Consume(
			q.Name, // queue
			"",     // consumer
			true,   // auto-ack
			false,  // exclusive
			false,  // no-local
			false,  // no-wait
			nil,    // args
		)
		if err == nil {
			break
		}
		log.Printf("Failed to register a consumer (attempt %d/%d): %v\n", i+1, maxRetries, err)
		time.Sleep(2 * time.Second) // Wait before retrying
	}
	if err != nil {
		log.Fatalf("Failed to register a consumer after %d retries: %v", maxRetries, err)
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
