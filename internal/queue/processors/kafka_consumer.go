package processors

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func ConsumeKafkaMessages(bootStrapServer string, groupId string, autoOffset string, topicName string) <-chan *kafka.Message {
	messages := make(chan *kafka.Message)

	// Kafka consumer configuration
	config := kafka.ConfigMap{
		"bootstrap.servers":  bootStrapServer,
		"group.id":           groupId,
		"auto.offset.reset":  autoOffset,
		"enable.auto.commit": "false",
	}

	// Create Kafka consumer
	consumer, err := kafka.NewConsumer(&config)
	if err != nil {
		log.Fatalf("Failed to create consumer: %s\n", err)
	}
	defer consumer.Close()

	// Subscribe to topic(s)
	topic := topicName
	err = consumer.SubscribeTopics([]string{topic}, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to topic: %s\n", err)
	}

	// Channel to handle OS signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming messages
	go func() {
	ConsumerLoop:
		for {
			select {
			case sig := <-sigchan:
				log.Printf("Caught signal %v: terminating\n", sig)
				break ConsumerLoop
			default:
				msg, err := consumer.ReadMessage(-1)
				if err == nil {
					log.Printf("Received message: %s\n", string(msg.Value))
					messages <- msg
				} else {
					fmt.Printf("Consumer error: %v (%v)\n", err, msg)
				}
			}
		}
	}()

	return messages
}

