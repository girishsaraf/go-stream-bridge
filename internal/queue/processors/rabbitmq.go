package processors

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"github.com/streadway/amqp"
)

func ConsumeAMQPMessages(amqpServer string, queueName string) <-chan []byte {
	messages := make(chan []byte)

	// Connect to RabbitMQ server
	conn, err := amqp.Dial(amqpServer)
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
		queueName, // name
		false,        // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
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
